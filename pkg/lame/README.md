lame
====

Simple libmp3lame powered mp3 encoder for Go

**Note:** this project is obsolete, consider moving to https://github.com/viert/go-lame

Example:

```Go
package main

import (
  "bufio"
  "lame"
  "os"
)

func main() {
  f, err := os.Open("input.raw")
  if err != nil {
    panic(err)
  }
  defer f.Close()
  reader := bufio.NewReader(f)

  of, err := os.Create("output.mp3")
  if err != nil {
    panic(err)
  }
  defer of.Close()

  wr := lame.NewWriter(of)
  wr.Encoder.SetBitrate(112)
  wr.Encoder.SetQuality(1)

  // IMPORTANT!
  wr.Encoder.InitParams()

  reader.WriteTo(wr)

}
```
