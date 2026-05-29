import { Callout, Code, H2, P, Pre, UL } from "../components"

export function Chapter02() {
  return (
    <>
      <P>
        RESP (REdis Serialization Protocol) is a tiny line-oriented binary format. Commands are
        encoded as arrays of bulk strings. Replies use a small set of frame types. The whole
        protocol fits on a postcard, which is one reason Redis-compatible servers like MnemoKV
        can interoperate with any RESP client, including <Code>redis-cli</Code>.
      </P>

      <H2>Frame types</H2>
      <UL>
        <li>
          <Code>+OK\r\n</Code> — simple string
        </li>
        <li>
          <Code>-ERR ...\r\n</Code> — error
        </li>
        <li>
          <Code>:42\r\n</Code> — integer
        </li>
        <li>
          <Code>$5\r\nhello\r\n</Code> — bulk string with explicit length
        </li>
        <li>
          <Code>*2\r\n$3\r\nGET\r\n$3\r\nkey\r\n</Code> — array of frames
        </li>
        <li>
          <Code>$-1\r\n</Code> — null bulk
        </li>
      </UL>

      <H2>What a command looks like on the wire</H2>
      <P>
        When you type <Code>SET foo bar</Code> in <Code>redis-cli</Code>, the client sends:
      </P>
      <Pre>{`*3\\r\\n$3\\r\\nSET\\r\\n$3\\r\\nfoo\\r\\n$3\\r\\nbar\\r\\n`}</Pre>
      <P>The server replies:</P>
      <Pre>{`+OK\\r\\n`}</Pre>

      <H2>Why the protocol matters</H2>
      <UL>
        <li>It separates parsing from execution — the server can pool buffers and reuse them.</li>
        <li>It is fully request/response, so a single TCP connection can pipeline thousands of commands.</li>
        <li>It is human-readable enough that you can debug a session with <Code>nc</Code> or <Code>tcpdump</Code>.</li>
      </UL>

      <Callout>
        MnemoKV implements RESP2 in <Code>internal/resp/</Code>. The parser produces a
        <Code>Command</Code> struct, and a separate writer encodes reply frames. The two halves
        are deliberately independent so the engine can be tested without touching sockets.
      </Callout>
    </>
  )
}
