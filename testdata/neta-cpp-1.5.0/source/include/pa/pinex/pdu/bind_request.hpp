#pragma once

#include <pa/pinex/common.hpp>

namespace pa::pinex
{
struct bind_request
{
    pinex::bind_type bind_type{bind_type::bi_direction};
    std::string system_id{};

    bool operator==(const bind_request&) const = default;
};

namespace detail
{
inline auto command_id_of([[maybe_unused]] const bind_request& bind_request)
{
    return command_id::bind_req;
}

template<>
inline consteval auto members<bind_request>()
{
    using o = bind_request;
    // bind_type is now a real wire field (Finding C-4) -- placed first,
    // matching both the struct's own declaration order and the convention
    // of fixed-size fields preceding variable-length ones. This is an
    // intentional, documented wire-format change (see
    // docs/behavior/compatibility-contract.md): bind_type previously
    // contributed zero bytes and was always hardcoded to bi_direction on
    // receipt regardless of what was sent.
    return std::tuple{
        meta<enum_u8>(&o::bind_type, "bind_type"),
        meta<c_octet_str<20>>(&o::system_id, "system_id")};
}
}  // namespace detail
}  // namespace pa::pinex