import type { ReactNode } from "react"
import { Chapter01 } from "./01-what-is-inmemory"
import { Chapter02 } from "./02-resp-protocol"
import { Chapter03 } from "./03-strings"
import { Chapter04 } from "./04-lists"
import { Chapter05 } from "./05-sorted-sets"
import { Chapter06 } from "./06-lock-striping"
import { Chapter07 } from "./07-eviction"
import { Chapter08 } from "./08-sharding"
import { Chapter09 } from "./09-replication"
import { Chapter10 } from "./10-gossip"
import { Chapter11 } from "./11-failover"
import { Chapter12 } from "./12-benchmarks"

export type Chapter = {
  slug: string
  title: string
  summary: string
  body: () => ReactNode
}

export const chapters: Chapter[] = [
  {
    slug: "what-is-in-memory",
    title: "What is an in-memory database?",
    summary: "RAM versus disk, trade-offs, and when you reach for one.",
    body: Chapter01,
  },
  {
    slug: "resp-protocol",
    title: "The RESP2 protocol",
    summary: "How clients and servers exchange commands and replies.",
    body: Chapter02,
  },
  {
    slug: "strings",
    title: "Data structures: Strings",
    summary: "Hash maps, O(1) access, and how SET/GET/INCR work.",
    body: Chapter03,
  },
  {
    slug: "lists",
    title: "Data structures: Lists",
    summary: "Doubly linked lists for O(1) push and pop at both ends.",
    body: Chapter04,
  },
  {
    slug: "sorted-sets",
    title: "Data structures: Sorted Sets",
    summary: "Skip lists, O(log n) insert and range, and score ordering.",
    body: Chapter05,
  },
  {
    slug: "lock-striping",
    title: "Concurrency: Lock Striping",
    summary: "Splitting a global lock into many smaller ones for throughput.",
    body: Chapter06,
  },
  {
    slug: "eviction",
    title: "Memory Management & Eviction",
    summary: "Memory limits and FIFO, LRU, LFU, and Random policies.",
    body: Chapter07,
  },
  {
    slug: "sharding",
    title: "Consistent Hashing & Sharding",
    summary: "Distributing keys across nodes with minimal reshuffling.",
    body: Chapter08,
  },
  {
    slug: "replication",
    title: "Replication",
    summary: "Async versus strong modes and follower apply paths.",
    body: Chapter09,
  },
  {
    slug: "gossip",
    title: "Cluster Health: Gossip Protocol",
    summary: "How nodes detect each other's status and state transitions.",
    body: Chapter10,
  },
  {
    slug: "failover",
    title: "Manual Failover & Repair",
    summary: "Terms, stale-leader fencing, replica assignment, and full-slot recovery.",
    body: Chapter11,
  },
  {
    slug: "benchmarks",
    title: "Benchmarking",
    summary: "Reading ns/op, measuring properly, and interpreting results.",
    body: Chapter12,
  },
]
