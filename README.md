# HFS

**HFS** is a lightweight Go client for interacting with [Hugging Face Spaces](https://huggingface.co/spaces) that expose Gradio-style APIs.

It simplifies this workflow by providing a single `.Do()` method that:
1. Sends your inputs  
2. Retrieves the event ID  
3. Fetches the final result stream  
4. Returns typed output  

---

## ✨ Features

- 🌐 Connect to any Hugging Face Spaces endpoint  
- 🔁 Handles event ID + result fetching  
- 🔐 Supports custom headers and bearer tokens  
- ⚙️ Generic over input and output types  
- 🧩 FileData support for inputs and outputs  
- 🧼 Minimal API — just call `.Do()`
- 🛡️ No dependencies outside the standard library

---

## Installation

```bash
go get github.com/ucukertz/hfs
```

---

## Simple Usage Example

```go
package main

import (
    "fmt"
    "github.com/ucukertz/hfs"
)

func main() {
    space := hfs.NewHfs[string, string]("your-space-name").
        WithBearerToken("your-hf-token")

    result, err := space.Do("/your-endpoint", "your-input-string")
    if err != nil {
        panic(err)
    }

    for _, item := range result {
        fmt.Println(item)
    }
}
```

---

## Using `any` for Mixed Input/Output Types

```go
package main

import (
    "fmt"
    "github.com/ucukertz/hfs"
)

func main() {
    space := hfs.NewHfs[any, any]("your-space-name").
        WithBearerToken("your-hf-token")

    output, err := space.Do(
        "/your-endpoint",
        "some text",
        123,
        true,
        4.5,
        map[string]any{"key": "value"},
    )
    if err != nil {
        panic(err)
    }

    fmt.Println(output)
}
```

---

## Using `FileData` as Hugging Face Spaces Input

```go
package main

import (
    "fmt"
    "github.com/ucukertz/hfs"
)

func main() {
    fileInput, err := hfs.NewFileData("input.jpg").
        FromUrl("https://example.com/image.jpg")
    if err != nil {
        panic(err)
    }

    space := hfs.NewHfs[any, any]("your-space-name").
        WithBearerToken("your-hf-token")

    output, err := space.Do(
        "/your-endpoint",
        fileInput,
        "your-prompt",
        42,
    )
    if err != nil {
        panic(err)
    }

    fmt.Println(output)
}
```

---

## Handling `FileData` Output

```go
package main

import (
    "os"
    "github.com/ucukertz/hfs"
)

func main() {
    space := hfs.NewHfs[any, any]("your-space-name").
        WithBearerToken("your-hf-token")

    out, err := space.Do("/your-endpoint", "your-input")
    if err != nil {
        panic(err)
    }

    bytes, err := hfs.GetFileData(out[0])
    if err != nil {
        panic(err)
    }

    os.WriteFile("output.png", bytes, 0644)
}
```

You can use `DoFD()` instead of `Do()` when you only want to use []byte from an output FileData and the FileData is the first item of the output.

---

## Notes

- `GetFileData()` automatically extracts and downloads the content of a `FileData` output.
- Use `FileData.FromUrl()`, `.FromBytes()`, or `.FromBase64()` to construct uploadable inputs.
- `.WithBearerToken()`, `.WithTimeout()`, `.WithUserAgent()`, and `.WithHTTPClient()` allow full customization.
- This module uses the "curl" API so public URL for file input is [mandatory](https://www.gradio.app/guides/querying-gradio-apps-with-curl) (see "Files" section). `FileData.FromBytes()` and `.FromBase64()` use `Quax` to conveniently achieve this.
- `NewHfsRaw()` can be used for spaces not hosted on Hugging Face. 

---

## License

Distributed under the MIT License. See `LICENSE` for more information.

## Author

Created by **[ucukertz](https://github.com/ucukertz)**.
