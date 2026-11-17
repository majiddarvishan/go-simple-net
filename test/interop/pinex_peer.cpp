#include <pa/pinex/messaging_client.hpp>
#include <pa/pinex/messaging_server.hpp>

#include <boost/asio.hpp>

#include <algorithm>
#include <atomic>
#include <chrono>
#include <cstdint>
#include <iostream>
#include <memory>
#include <string>
#include <thread>
#include <vector>

namespace asio = boost::asio;
using namespace pa::pinex;
using namespace std::chrono_literals;

namespace
{
auto noop_packet()
{
    return [](const std::string&, uint32_t, const std::string&) {};
}

int run_server(const std::string& address, bool prefix_response)
{
    // Pinex's public Client/Server and Session APIs are single-io_context-
    // thread objects. Construct, start, dispatch callbacks and stop on this
    // same thread; io.run() therefore runs on main rather than a worker.
    asio::io_context io;
    asio::steady_timer deadline(io);
    bool completed = false;
    bool timed_out = false;
    std::unique_ptr<messaging_server> server;

    server = std::make_unique<messaging_server>(
        &io, "pinex-cpp-server", address, 5, 0,
        [&](const std::string& client_id, uint32_t sequence, const std::string& body) {
            try
            {
                const auto response = prefix_response ? "cpp:" + body : body;
                if (!server || server->send_response(response, sequence, client_id) != sequence)
                    std::cerr << "Pinex server failed to admit response\n";
            }
            catch (const std::exception& error)
            {
                std::cerr << "Pinex server response error: " << error.what() << "\n";
            }
        },
        noop_packet(),
        noop_packet(),
        [&](const std::string&, session_stat state) {
            if (state == session_stat::close && !completed)
            {
                completed = true;
                if (server)
                    server->stop();
                deadline.cancel();
            }
        });

    deadline.expires_after(15s);
    deadline.async_wait([&](const boost::system::error_code& error) {
        if (!error)
        {
            timed_out = true;
            if (server)
                server->stop();
        }
    });

    std::cout << "READY\n" << std::flush;
    io.run();
    return completed && !timed_out ? 0 : 2;
}

int run_client(const std::string& address)
{
    // Keep all Pinex operations on the same thread that first calls start().
    asio::io_context io;
    asio::steady_timer deadline(io);
    bool completed = false;
    bool timed_out = false;
    bool failed = false;
    bool outcome_set = false;
    std::string outcome;
    std::shared_ptr<messaging_client> client;

    auto publish_outcome = [&](std::string value) {
        if (!outcome_set)
        {
            outcome = std::move(value);
            outcome_set = true;
        }
    };

    client = std::make_shared<messaging_client>(
        &io, "pinex-cpp-client", address, 5, 0, false,
        noop_packet(),
        [&](const std::string&, uint32_t, const std::string& body) {
            publish_outcome(body);
            if (client)
                client->stop();
        },
        [&](const std::string&, uint32_t, const std::string& body) {
            publish_outcome("TIMEOUT:" + body);
            failed = true;
            asio::post(io, [&] {
                if (client)
                    client->stop();
            });
        },
        nullptr,
        nullptr,
        [&](const std::string&, session_stat state) {
            if (state == session_stat::bind)
            {
                if (!client || client->send_request("from-cpp") == 0)
                {
                    publish_outcome("SEND_FAILED");
                    failed = true;
                    asio::post(io, [&] {
                        if (client)
                            client->stop();
                    });
                }
            }
            else if (state == session_stat::close)
            {
                completed = true;
                deadline.cancel();
            }
        },
        [&](const std::string& error) {
            publish_outcome("ERROR:" + error);
            failed = true;
            asio::post(io, [&] {
                if (client)
                    client->stop();
                deadline.cancel();
            });
        },
        2s,
        2s);

    deadline.expires_after(15s);
    deadline.async_wait([&](const boost::system::error_code& error) {
        if (!error)
        {
            timed_out = true;
            if (client)
                client->stop();
        }
    });

    const auto stop_at = std::chrono::steady_clock::now() + 15s;
    while (!completed && !timed_out && !failed &&
           std::chrono::steady_clock::now() < stop_at)
    {
        if (io.stopped())
            io.restart();
        io.run_one_for(5ms);
    }
    if (!completed && !timed_out && !failed)
    {
        timed_out = true;
        if (client)
            client->stop();
    }
    if (failed && client)
        client->stop();
    io.poll();
    io.stop();

    if (timed_out || !completed)
    {
        std::cerr << "Pinex Client did not complete the request/Unbind exchange\n";
        return 2;
    }
    if (!outcome_set || outcome != "go:from-cpp")
    {
        std::cerr << "Unexpected Go response: " << outcome << "\n";
        return 3;
    }

    std::cout << "PASS " << outcome << "\n";
    return 0;
}

int run_client_bench(const std::string& address, int target, int payload_size)
{
    if (target < 1 || payload_size < 0 || payload_size > (1 << 20))
        return 64;

    asio::io_context io;
    asio::steady_timer deadline(io);
    std::shared_ptr<messaging_client> client;
    std::thread gate_thread;
    std::atomic<bool> gate_done{false};
    bool completed = false;
    bool timed_out = false;
    bool failed = false;
    int responses = 0;
    std::string current_body;
    std::vector<int64_t> latencies;
    latencies.reserve(static_cast<std::size_t>(target));
    std::chrono::steady_clock::time_point started{};
    std::chrono::steady_clock::time_point request_started{};
    int64_t total_elapsed_ns = 0;

    auto send_next = [&]() {
        current_body.assign(static_cast<std::size_t>(payload_size), 'x');
        request_started = std::chrono::steady_clock::now();
        if (!client || client->send_request(current_body) == 0)
        {
            failed = true;
            if (client)
                client->stop();
        }
    };

    client = std::make_shared<messaging_client>(
        &io, "pinex-cpp-bench-client", address, 5, 0, false,
        noop_packet(),
        [&](const std::string&, uint32_t, const std::string& body) {
            const auto now = std::chrono::steady_clock::now();
            if (body != current_body)
            {
                failed = true;
                if (client)
                    client->stop();
                return;
            }
            latencies.push_back(std::chrono::duration_cast<std::chrono::nanoseconds>(
                                    now - request_started)
                                    .count());
            ++responses;
            if (responses < target)
            {
                send_next();
            }
            else
            {
                total_elapsed_ns = std::chrono::duration_cast<std::chrono::nanoseconds>(
                                       now - started)
                                       .count();
                if (client)
                    client->stop();
            }
        },
        [&](const std::string&, uint32_t, const std::string&) {
            failed = true;
            asio::post(io, [&] {
                if (client)
                    client->stop();
                deadline.cancel();
            });
        },
        nullptr,
        nullptr,
        [&](const std::string&, session_stat state) {
            if (state == session_stat::bind)
            {
                std::cout << "READY\n" << std::flush;
                gate_thread = std::thread([&] {
                    std::string token;
                    std::getline(std::cin, token);
                    gate_done.store(true);
                    asio::post(io, [&, token = std::move(token)] {
                        if (token == "GO")
                        {
                            started = std::chrono::steady_clock::now();
                            send_next();
                        }
                        else
                        {
                            failed = true;
                            if (client)
                                client->stop();
                        }
                    });
                });
            }
            else if (state == session_stat::close)
            {
                completed = true;
                deadline.cancel();
            }
        },
        [&](const std::string&) {
            failed = true;
            asio::post(io, [&] {
                if (client)
                    client->stop();
                deadline.cancel();
            });
        },
        2s,
        2s);

    deadline.expires_after(60s);
    deadline.async_wait([&](const boost::system::error_code& error) {
        if (!error)
        {
            timed_out = true;
            if (client)
                client->stop();
        }
    });

    const auto stop_at = std::chrono::steady_clock::now() + 60s;
    while (!completed && !timed_out && !failed &&
           std::chrono::steady_clock::now() < stop_at)
    {
        if (io.stopped())
            io.restart();
        io.run_one_for(5ms);
    }
    if (!completed && !timed_out && !failed)
    {
        timed_out = true;
        if (client)
            client->stop();
    }
    if ((failed || timed_out) && client)
        client->stop();
    io.poll();
    io.stop();

    if (gate_thread.joinable())
    {
        if (gate_done.load())
            gate_thread.join();
        else
            gate_thread.detach();
    }

    if (failed || timed_out || !completed || responses != target ||
        latencies.size() != static_cast<std::size_t>(target) || total_elapsed_ns <= 0)
    {
        std::cerr << "Pinex C++ Client benchmark did not complete cleanly; responses="
                  << responses << " target=" << target << "\n";
        return 2;
    }

    std::sort(latencies.begin(), latencies.end());
    const auto percentile = [&](double quantile) {
        const auto index = static_cast<std::size_t>(
            quantile * static_cast<double>(latencies.size() - 1));
        return latencies[index];
    };
    std::cout << "PASS " << responses << " " << total_elapsed_ns << " "
              << percentile(0.50) << " " << percentile(0.95) << " "
              << percentile(0.99) << "\n";
    return 0;
}
} // namespace

int main(int argc, char** argv)
{
    if (argc < 3)
    {
        std::cerr << "usage: pinex-peer server|server-echo|client host:port | client-bench host:port count payload-size\n";
        return 64;
    }

    const std::string mode{argv[1]};
    const std::string address{argv[2]};
    try
    {
        if (mode == "client-bench")
        {
            if (argc != 5)
                return 64;
            return run_client_bench(address, std::stoi(argv[3]), std::stoi(argv[4]));
        }
        if (argc != 3)
            return 64;
        if (mode == "server")
            return run_server(address, true);
        if (mode == "server-echo")
            return run_server(address, false);
        if (mode == "client")
            return run_client(address);
    }
    catch (const std::exception& error)
    {
        std::cerr << "Pinex peer exception: " << error.what() << "\n";
        return 70;
    }

    std::cerr << "unknown mode: " << mode << "\n";
    return 64;
}
