# Tests for Dyson's Python-like time compatibility module.
#
# This file is chunked by lines containing "---". Each chunk executes as an
# independent Starlark file so related assertions stay small and failures point
# at a focused behavior area. The Go harness runs these chunks inside
# testing/synctest, so time starts at 2000-01-01 00:00:00 UTC and sleep advances
# fake time deterministically.

---
# The module is imported as a namespace symbol, matching Dyson stdlib load
# semantics and Python-style call sites.
load("assert.star", "assert")
load("time.star", "time")

assert.true("time" in dir(time), "time")
assert.true("sleep" in dir(time), "sleep")
assert.true("gmtime" in dir(time), "gmtime")
assert.true("strftime" in dir(time), "strftime")
assert.true("struct_time" in dir(time), "struct_time")

---
# Timezone globals are present and default to UTC for deterministic execution.
load("assert.star", "assert")
load("time.star", "time")

assert.eq(time.timezone, 0)
assert.eq(time.altzone, 0)
assert.eq(time.daylight, 0)
assert.eq(time.tzname, ("UTC", "UTC"))

---
# Wall-clock clocks report the synctest fake Unix time.
load("assert.star", "assert")
load("time.star", "time")

assert.eq(time.time(), 946684800.0)
assert.eq(time.time_ns(), 946684800000000000)

---
# sleep advances wall-clock time deterministically inside synctest.
load("assert.star", "assert")
load("time.star", "time")

start = time.time_ns()
assert.eq(time.sleep(1.25), None)
assert.eq(time.time_ns() - start, 1250000000)
assert.eq(time.time(), 946684801.25)

---
# monotonic clocks start at zero for each test bubble and advance with sleep.
load("assert.star", "assert")
load("time.star", "time")

assert.eq(time.monotonic(), 0.0)
assert.eq(time.monotonic_ns(), 0)
time.sleep(2.5)
assert.eq(time.monotonic(), 2.5)
assert.eq(time.monotonic_ns(), 2500000000)

---
# perf_counter uses the same monotonic elapsed-time source as monotonic.
load("assert.star", "assert")
load("time.star", "time")

assert.eq(time.perf_counter(), 0.0)
assert.eq(time.perf_counter_ns(), 0)
time.sleep(0.000001)
assert.eq(time.perf_counter_ns(), 1000)
assert.eq(time.perf_counter(), 0.000001)

---
# process_time is intentionally unsupported for now. Python defines it as
# process-wide CPU time, not wall-clock or monotonic time, and Go has no portable
# standard-library equivalent.
load("time.star", "time")

time.process_time() ### "time.process_time: CPU time clocks are not supported"

---
# process_time_ns is intentionally unsupported for the same reason as
# process_time.
load("time.star", "time")

time.process_time_ns() ### "time.process_time_ns: CPU time clocks are not supported"

---
# thread_time is intentionally unsupported for now. Python defines it as current
# OS-thread CPU time, but Go Starlark execution runs on goroutines that may move
# across OS threads.
load("time.star", "time")

time.thread_time() ### "time.thread_time: CPU time clocks are not supported"

---
# thread_time_ns is intentionally unsupported for the same reason as
# thread_time.
load("time.star", "time")

time.thread_time_ns() ### "time.thread_time_ns: CPU time clocks are not supported"

---
# gmtime converts epoch seconds to a UTC struct_time-like value with Python field
# order and attributes.
load("assert.star", "assert")
load("time.star", "time")

utc = time.gmtime(0)
assert.eq(tuple(utc), (1970, 1, 1, 0, 0, 0, 3, 1, 0))
assert.eq((utc.tm_year, utc.tm_mon, utc.tm_mday, utc.tm_hour, utc.tm_min, utc.tm_sec), (1970, 1, 1, 0, 0, 0))
assert.eq((utc.tm_wday, utc.tm_yday, utc.tm_isdst), (3, 1, 0))
assert.eq(utc[0], 1970)
assert.eq(len(utc), 9)

---
# gmtime with no argument defaults to the current wall-clock time.
load("assert.star", "assert")
load("time.star", "time")

now = time.gmtime()
assert.eq(tuple(now), (2000, 1, 1, 0, 0, 0, 5, 1, 0))

---
# localtime uses UTC in Dyson's deterministic stdlib policy.
load("assert.star", "assert")
load("time.star", "time")

assert.eq(tuple(time.localtime(0)), (1970, 1, 1, 0, 0, 0, 3, 1, 0))
assert.eq(tuple(time.localtime()), (2000, 1, 1, 0, 0, 0, 5, 1, 0))

---
# mktime converts a local-time tuple back to epoch seconds. With Dyson's UTC
# policy this is the inverse of gmtime/localtime for supported tuples.
load("assert.star", "assert")
load("time.star", "time")

assert.eq(time.mktime((1970, 1, 1, 0, 0, 0, 3, 1, 0)), 0.0)
assert.eq(time.mktime((2000, 1, 1, 0, 0, 0, 5, 1, 0)), 946684800.0)

---
# asctime formats a 9-item time tuple with CPython's fixed-width weekday and
# month representation.
load("assert.star", "assert")
load("time.star", "time")

assert.eq(time.asctime((1970, 1, 1, 0, 0, 0, 3, 1, 0)), "Thu Jan  1 00:00:00 1970")
assert.eq(time.asctime((2000, 1, 2, 3, 4, 5, 6, 2, 0)), "Sun Jan  2 03:04:05 2000")

---
# asctime with no argument defaults to localtime().
load("assert.star", "assert")
load("time.star", "time")

assert.eq(time.asctime(), "Sat Jan  1 00:00:00 2000")

---
# ctime is equivalent to asctime(localtime(seconds)).
load("assert.star", "assert")
load("time.star", "time")

assert.eq(time.ctime(0), "Thu Jan  1 00:00:00 1970")
assert.eq(time.ctime(), "Sat Jan  1 00:00:00 2000")

---
# strftime formats a tuple or struct_time using common Python-compatible format
# directives.
load("assert.star", "assert")
load("time.star", "time")

assert.eq(time.strftime("%Y-%m-%d %H:%M:%S", (1970, 1, 1, 0, 0, 0, 3, 1, 0)), "1970-01-01 00:00:00")
assert.eq(time.strftime("%a %b %d %H:%M:%S %Y", time.gmtime(0)), "Thu Jan 01 00:00:00 1970")
assert.eq(time.strftime("%j %w %%", time.gmtime(0)), "001 4 %")

---
# strftime with no time argument defaults to localtime().
load("assert.star", "assert")
load("time.star", "time")

assert.eq(time.strftime("%Y"), "2000")

---
# strptime parses strings into a struct_time-like value using common directives.
load("assert.star", "assert")
load("time.star", "time")

parsed = time.strptime("1970-01-01 00:00:00", "%Y-%m-%d %H:%M:%S")
assert.eq(tuple(parsed), (1970, 1, 1, 0, 0, 0, 3, 1, 0))

---
# get_clock_info returns a namespace-like object with Python-compatible fields.
load("assert.star", "assert")
load("time.star", "time")

wall = time.get_clock_info("time")
mono = time.get_clock_info("monotonic")
perf = time.get_clock_info("perf_counter")
assert.eq((wall.monotonic, wall.adjustable), (False, True))
assert.eq((mono.monotonic, mono.adjustable), (True, False))
assert.eq((perf.monotonic, perf.adjustable), (True, False))
assert.true(wall.resolution > 0.0, "wall resolution")
assert.true("dyson" in wall.implementation, "implementation")

---
# tzset is intentionally unsupported because Dyson's time module uses a
# deterministic UTC timezone policy rather than reading process environment.
load("time.star", "time")

time.tzset() ### "time.tzset: timezone environment changes are not supported"

---
# sleep rejects negative durations and non-numeric values.
load("time.star", "time")

time.sleep(-1) ### "time.sleep: sleep length must be non-negative"

---
# sleep validates argument type before trying to block.
load("time.star", "time")

time.sleep("not seconds") ### "time.sleep: seconds must be int or float, got string"

---
# strptime reports invalid input as a function-local error.
load("time.star", "time")

time.strptime("not a date", "%Y-%m-%d") ### "time.strptime:"
