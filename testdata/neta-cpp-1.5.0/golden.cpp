// This executable uses the pinned C++ peer PDU serializers to
// generate cross-language golden frames consumed by internal/wire tests.
//
// The 10-byte header writer below mirrors session::serialize_header in
// the original include/pa/pinex/net/session.hpp at source commit
// f7bd1eb779baff0d10e624f20522df345e6ee747. Control PDUs are emitted by
// session::send_command using the same header writer and have no Body.
#include <pa/pinex/common.hpp>
#include <pa/pinex/pdu.hpp>

#include <cstdint>
#include <iomanip>
#include <iostream>
#include <string>
#include <vector>

namespace
{
using namespace pa::pinex;

std::vector<uint8_t> serialize_body(const auto& pdu)
{
    std::vector<uint8_t> body;
    serialize_to(&body, pdu);
    return body;
}

void put_u32_be(std::vector<uint8_t>& out, std::size_t offset, uint32_t value)
{
    out[offset + 0] = static_cast<uint8_t>((value >> 24) & 0xff);
    out[offset + 1] = static_cast<uint8_t>((value >> 16) & 0xff);
    out[offset + 2] = static_cast<uint8_t>((value >> 8) & 0xff);
    out[offset + 3] = static_cast<uint8_t>(value & 0xff);
}

std::vector<uint8_t> make_frame(command_id command,
                                command_status status,
                                uint32_t sequence,
                                const std::vector<uint8_t>& body)
{
    const auto length = static_cast<uint32_t>(10 + body.size());
    std::vector<uint8_t> frame(10);
    put_u32_be(frame, 0, length);
    frame[4] = static_cast<uint8_t>(command);
    frame[5] = static_cast<uint8_t>(status);
    put_u32_be(frame, 6, sequence);
    frame.insert(frame.end(), body.begin(), body.end());
    return frame;
}

void print_frame(const char* name, const std::vector<uint8_t>& frame)
{
    std::cout << name << '=';
    for (const auto byte : frame)
        std::cout << std::hex << std::setfill('0') << std::setw(2)
                  << static_cast<unsigned>(byte);
    std::cout << '\n';
}

int run()
{
    const bind_request bind_req{
        .bind_type = bind_type::bi_direction,
        .system_id = "node"
    };
    const auto bind_req_body = serialize_body(bind_req);
    if (deserialize<bind_request>(bind_req_body) != bind_req)
        return 1;
    print_frame("bind_request",
                make_frame(command_id::bind_req, command_status::rok, 7, bind_req_body));

    const bind_response bind_resp{
        .bind_type = bind_type::uni_direction,
        .system_id = "srv"
    };
    const auto bind_resp_body = serialize_body(bind_resp);
    if (deserialize<bind_response>(bind_resp_body) != bind_resp)
        return 2;
    print_frame("bind_response",
                make_frame(command_id::bind_resp, command_status::rfail, 7, bind_resp_body));

    const stream_request stream_req{.message_body = std::string{"\xaa\xbb", 2}};
    const auto stream_req_body = serialize_body(stream_req);
    if (deserialize<stream_request>(stream_req_body) != stream_req)
        return 3;
    print_frame("stream_request",
                make_frame(command_id::stream_req, command_status::rok,
                           0x01020304, stream_req_body));

    const stream_response stream_resp{
        .message_body = std::string{"\x00\x01\x02\xfe\xff", 5}
    };
    const auto stream_resp_body = serialize_body(stream_resp);
    if (deserialize<stream_response>(stream_resp_body) != stream_resp)
        return 4;
    print_frame("stream_response",
                make_frame(command_id::stream_resp, command_status::rfail,
                           42, stream_resp_body));

    // The pinned C++ peer's bind_response specialization permits an omitted/empty Body
    // and returns the struct's default values.
    const std::vector<uint8_t> empty_body;
    const auto omitted_bind_resp = deserialize<bind_response>(empty_body);
    if (omitted_bind_resp.bind_type != bind_type::bi_direction
        || !omitted_bind_resp.system_id.empty())
        return 5;
    print_frame("bind_response_empty",
                make_frame(command_id::bind_resp, command_status::rfail, 7, empty_body));

    const std::vector<uint8_t> no_body;
    print_frame("unbind_request",
                make_frame(command_id::unbind_req, command_status::rok,
                           0x12345678, no_body));
    print_frame("unbind_response",
                make_frame(command_id::unbind_resp, command_status::rok,
                           0x12345678, no_body));
    print_frame("enquire_link_request",
                make_frame(command_id::enquire_link_req, command_status::rok,
                           0x12345678, no_body));
    print_frame("enquire_link_response",
                make_frame(command_id::enquire_link_resp, command_status::rok,
                           0x12345678, no_body));
    return 0;
}
}  // namespace

int main()
{
    return run();
}
