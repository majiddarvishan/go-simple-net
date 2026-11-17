#pragma once

#include <cassert>
#include <optional>
#include <thread>

namespace pa::pinex::detail
{
/// @brief Debug-only executor-affinity check for classes documented as
/// "single thread only, no internal synchronization" (see
/// docs/thread-safety.md). Lazily records the first thread observed
/// calling `check()` and asserts every subsequent call comes from that
/// same thread.
///
/// Deliberately NOT based on `io_context::running_in_this_thread()`:
/// Asio implements that as `thread_call_stack::contains(this)`, which is
/// only true while a `run()`/`poll()` call for that `io_context` is
/// *currently active on the calling thread's stack* -- false the instant
/// `run()` returns, including for direct calls made between event-loop
/// pumps on the correct thread (exactly how this project's own test suite
/// calls into `session`/`p_client`/etc. throughout: directly, after
/// `run_until()` returns, not from inside a handler). Tracking "which
/// single thread has touched this object" is the check that actually
/// matches the documented single-thread contract.
///
/// `check()`'s entire body -- including the `std::optional` read/write --
/// lives inside `assert()`'s argument, which the preprocessor deletes
/// entirely when `NDEBUG` is defined (per the standard `<cassert>` macro),
/// so this is genuinely zero cost in release builds. This is a debug aid,
/// not a runtime guard -- do not rely on it for anything security- or
/// correctness-critical in release builds.
class thread_affinity_guard
{
#ifndef NDEBUG
    mutable std::optional<std::thread::id> owner_thread_id_;
#endif

  public:
    void check() const
    {
#ifndef NDEBUG
        assert([&] {
            auto this_thread = std::this_thread::get_id();
            if (!owner_thread_id_)
                owner_thread_id_ = this_thread;
            return *owner_thread_id_ == this_thread;
        }());
#endif
    }
};
}  // namespace pa::pinex::detail
