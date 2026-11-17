#include <pa/pinex/messaging_client.hpp>
#include <pa/pinex/messaging_server.hpp>

#include <boost/asio.hpp>

#include <chrono>
#include <iostream>
#include <memory>
#include <string>

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

    io.run();
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
