# slowtest

`go test -json` gives you a firehose of newline-delimited JSON events. When a
test suite gets slow and you want to know which tests are actually costing
you time, you end up writing the same `jq` one-liner every time (or worse,
scrolling through plain text output looking for big numbers). slowtest does
that one job: read the event stream and print the slowest tests.

## Usage

Pipe test output straight in:

```
go test -json ./... | slowtest
```

Or run against a saved log:

```
go test -json ./... > out.json
slowtest out.json
```

`-` also means stdin, so it works in places that expect an explicit input
argument:

```
go test -json ./... | slowtest -n 20 -
```

Output is one line per test, slowest first:

```
   1.842s  pass  example.com/pkg/store  TestStore_ConcurrentWrites
   0.910s  pass  example.com/pkg/store  TestStore_LargeBatch
   0.203s  fail  example.com/pkg/api    TestAPI_Timeout
```

By default it prints the top 10. Use `-n` to change that, or `-n 0` to print
every test that ran.

Pass `-json` to get the same results as a JSON array instead, for feeding
into another tool:

```
go test -json ./... | slowtest -json
```

```json
[
  {
    "Package": "example.com/pkg/store",
    "Test": "TestStore_ConcurrentWrites",
    "Action": "pass",
    "Elapsed": 1.842
  }
]
```

## Building

```
go build -o slowtest .
```

No third-party dependencies, standard library only.

## Notes

Only `pass`, `fail`, and `skip` events with a `Test` field are counted; build
output and package-level events are ignored. If a package fails to build,
`go test -json` won't emit per-test events for it, so those failures won't
show up here.
