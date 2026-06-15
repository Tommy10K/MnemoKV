import { Callout, Code, H2, P, Pre, UL } from "../components"

export function Chapter12() {
  return (
    <>
      <P>
        A benchmark answers a single question: how much does this operation cost? In Go, the
        cost is reported in nanoseconds per operation (<Code>ns/op</Code>), bytes allocated per
        operation (<Code>B/op</Code>), and allocations per operation (<Code>allocs/op</Code>).
        Each number tells you something different.
      </P>

      <H2>What the numbers mean</H2>
      <UL>
        <li>
          <Code>ns/op</Code> — wall-clock time per operation. Lower is faster.
        </li>
        <li>
          <Code>B/op</Code> — total heap bytes allocated per operation. Lower means less GC
          pressure.
        </li>
        <li>
          <Code>allocs/op</Code> — number of distinct allocations. Even small allocations cost,
          because each one is a chance for the GC to follow a pointer.
        </li>
      </UL>

      <H2>An example line</H2>
      <Pre>{`BenchmarkSET-16    3000000    412 ns/op    48 B/op    1 allocs/op`}</Pre>
      <P>
        This illustrative line reports three million iterations at roughly 412 nanoseconds each,
        with 48 bytes and one heap allocation per call. It is not a current MnemoKV result.
      </P>

      <H2>How to measure honestly</H2>
      <UL>
        <li>Run the benchmark multiple times (<Code>-count=3</Code>) and compare.</li>
        <li>Warm up the CPU before reading numbers; early runs are often noisier or slower.</li>
        <li>Compare like with like — same key sizes, same machine, same Go version.</li>
        <li>Beware of the compiler optimizing your work away if you do not consume the result.</li>
      </UL>

      <H2>What you'll see in MnemoKV</H2>
      <UL>
        <li>Strings are expected to be a cheap baseline, but copies and metadata updates still matter.</li>
        <li>List pushes allocate linked nodes and may show more allocation pressure.</li>
        <li>Sorted-set writes maintain both an index and ordering structure.</li>
        <li>Eviction overhead is invisible until memory is actually full.</li>
      </UL>

      <Callout>
        The benchmark page in the Use section can import output produced by
        <Code>scripts/benchmark.sh</Code> and compare these numbers visually instead of squinting
        at raw output.
      </Callout>
    </>
  )
}
