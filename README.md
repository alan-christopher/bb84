# BB84

## Overview

The [BB84 protocol](https://arxiv.org/abs/1002.1237) leverages no-cloning
properties of quantum systems to allow two parties -- Alice and Bob -- to
negotiate a shared secret which is impervious to an attacker -- Eve -- with
unlimited mathematical sophistication and computational resources. This project
aims to provide an off-the-shelf implementation of the software side of the BB84
protocol and the post-processing steps necessary to make it practically useful.

## Project Status

Pre-pre-pre-alpha.

## Language Support

We currently only support Go, but plausibly might provide C++ and/or Rust
implementations in the future.

## Contributing

This is a free/libre project, so pull requests and bug reports are always
welcom. Would-be contributors should be aware of two caveats, however:

1. This project is currently funded on a single individual's hobby
   time. Expectations on turnaround should be set accordingly.
1. Pull requests which don't respect [Effective
   Go](https://golang.org/doc/effective_go), aren't formatted with `gofmt`, or
   don't contain sufficient tests shouldn't expect to be merged.
