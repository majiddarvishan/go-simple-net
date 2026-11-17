#pragma once

#include <cstdint>
#include <cstdio>
#include <functional>
#include <source_location>
#include <string>
#include <string_view>
#include <utility>

namespace pa::pinex
{
// A small, dependency-free logging contract. The source location always
// identifies the Pinex call site that produced the diagnostic, never the
// application adapter which eventually forwards it to pa_logger/spdlog/etc.
enum class log_level { trace, debug, info, warn, error };

struct log_event
{
    log_level level{log_level::error};
    std::string component;
    std::string phase;
    std::string message;
    std::string source_file;
    std::uint_least32_t source_line{};
};

// An empty handler retains Pinex's existing stderr behavior.
// An installed handler replaces stderr, it is not an additional destination.
// The application owns thread-safety if it shares one handler between runners.
using log_handler = std::function<void(const log_event&)>;

// Uppercase names match the existing [pinex] level=ERROR stderr format.
inline constexpr const char* log_level_name(log_level level) noexcept
{
    switch (level)
    {
        case log_level::trace: return "TRACE";
        case log_level::debug: return "DEBUG";
        case log_level::info:  return "INFO";
        case log_level::warn:  return "WARN";
        case log_level::error: return "ERROR";
    }
    return "UNKNOWN";
}

// Explicit severity for newly instrumented call sites. No logger library is
// required; the application chooses where events go and how to filter them.
inline void emit_log(
    const log_handler& logger,
    log_level level,
    std::string_view component,
    std::string_view phase,
    std::string message,
    std::source_location origin = std::source_location::current())
{
    if (logger)
    {
        try
        {
            logger(log_event{
                level,
                std::string(component),
                std::string(phase),
                std::move(message),
                origin.file_name(),
                origin.line()
            });
            return;
        }
        catch (...)
        {
            // A misbehaving application logger must not break the network
            // state machine. No user-supplied exception text or payloads
            // are printed here (avoid leaking credentials or recursion).
            std::fprintf(stderr,
                "[pinex] level=ERROR component=logging phase=callback"
                " reason=logger_callback_threw source=%s:%u\n",
                origin.file_name(), static_cast<unsigned>(origin.line()));
            std::fflush(stderr);
            return;
        }
    }

    std::fprintf(stderr, "[pinex] level=%s component=%.*s phase=%.*s %s\n",
        log_level_name(level),
        static_cast<int>(component.size()), component.data(),
        static_cast<int>(phase.size()), phase.data(),
        message.c_str());
    std::fflush(stderr);
}

// Backward-compatible overload: existing calls remain ERROR, with the
// ORIGINAL caller's source location rather than this forwarding overload's.
inline void emit_log(
    const log_handler& logger,
    std::string_view component,
    std::string_view phase,
    std::string message,
    std::source_location origin = std::source_location::current())
{
    emit_log(logger, log_level::error, component, phase,
             std::move(message), origin);
}
} // namespace pa::pinex
