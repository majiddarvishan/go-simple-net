#include <pa/pinex/messaging_client.hpp>
#include <pa/pinex/messaging_server.hpp>

#include <boost/asio.hpp>

#include <atomic>
#include <chrono>
#include <future>
#include <iostream>
#include <memory>
#include <string>
#include <thread>

namespace asio = boost::asio;
using namespace pa::pinex;
using namespace std::chrono_literals;

namespace
{
auto noop_packet()
{
    return [](const std::string&, uint32_t, const std::string&) {};
}

int run_server(const std::string& address)
{
    asio::io_context io;
    std::promise<void> stopped;
    auto stopped_future = stopped.get_future();
    std::atomic<bool> stopped_set{false};
    std::unique_ptr<messaging_server> server;

    server = std::make_unique<messaging_server>(
        &io, "pinex-cpp-server", address, 5, 0,
        [&](const std::string& client_id, uint32_t sequence, const std::string& body) {
            try
            {
                if (!server || server->send_response("cpp:" + body, sequence, client_id) != sequence)
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
            if (state == session_stat::close && !stopped_set.exchange(true))
            {
                asio::post(io, [&] {
                    if (server)
                        server->stop();
                    stopped.set_value();
                });
            }
        });

    std::thread runner([&] { io.run(); });
    std::cout << "READY\n" << std::flush;

    if (stopped_future.wait_for(15s) != std::future_status::ready)
    {
        if (!stopped_set.exchange(true))
        {
            asio::post(io, [&] {
                if (server)
                    server->stop();
                stopped.set_value();
            });
        }
        if (stopped_future.wait_for(2s) != std::future_status::ready)
            io.stop();
        runner.join();
        return 2;
    }

    io.stop();
    runner.join();
    return 0;
}

int run_client(const std::string& address)
{
    asio::io_context io;
    std::promise<std::string> outcome;
    auto outcome_future = outcome.get_future();
    std::promise<void> closed;
    auto closed_future = closed.get_future();
    std::atomic<bool> outcome_set{false};
    std::atomic<bool> closed_set{false};
    std::shared_ptr<messaging_client> client;

    auto publish_outcome = [&](std::string value) {
        if (!outcome_set.exchange(true))
            outcome.set_value(std::move(value));
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
        },
        nullptr,
        nullptr,
        [&](const std::string&, session_stat state) {
            if (state == session_stat::bind)
            {
                if (!client || client->send_request("from-cpp") == 0)
                    publish_outcome("SEND_FAILED");
            }
            else if (state == session_stat::close && !closed_set.exchange(true))
            {
                closed.set_value();
            }
        },
        [&](const std::string& error) {
            publish_outcome("ERROR:" + error);
        },
        2s,
        2s);

    std::thread runner([&] { io.run(); });

    if (outcome_future.wait_for(15s) != std::future_status::ready)
    {
        asio::post(io, [&] {
            if (client)
                client->stop();
        });
        if (closed_future.wait_for(2s) != std::future_status::ready)
            io.stop();
        runner.join();
        std::cerr << "Pinex Client timed out waiting for Go response\n";
        return 2;
    }

    const auto result = outcome_future.get();
    if (result != "go:from-cpp")
    {
        asio::post(io, [&] {
            if (client)
                client->stop();
        });
        closed_future.wait_for(2s);
        io.stop();
        runner.join();
        std::cerr << "Unexpected Go response: " << result << "\n";
        return 3;
    }

    if (closed_future.wait_for(5s) != std::future_status::ready)
    {
        io.stop();
        runner.join();
        std::cerr << "Pinex Client did not finish graceful Unbind\n";
        return 4;
    }

    io.stop();
    runner.join();
    std::cout << "PASS " << result << "\n";
    return 0;
}
} // namespace

int main(int argc, char** argv)
{
    if (argc != 3)
    {
        std::cerr << "usage: pinex-peer server|client host:port\n";
        return 64;
    }

    const std::string mode{argv[1]};
    const std::string address{argv[2]};
    try
    {
        if (mode == "server")
            return run_server(address);
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
