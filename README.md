# tg-cleaner

Leave every Telegram channel and group while retaining private conversations.

## Getting started

This application uses
[`zelenin/go-tdlib`](https://github.com/zelenin/go-tdlib), which requires CGO
and a native TDLib installation. Install TDLib by following the
[TDLib build instructions](https://tdlib.github.io/td/build.html).

Create a local configuration file:

```sh
cp config.example.yaml config.yaml
```

Set the Telegram credentials obtained from <https://my.telegram.org/apps>:

```yaml
telegram:
  app_id: 123456
  app_hash: "your-app-hash"
  phone: "+15551234567"
  password: "your-telegram-2fa-password"
```

`password` is optional unless two-step verification is enabled. Keep
`config.yaml` private; it is ignored by Git.

Run a safe preview first:

```sh
go run .
```

The preview lists the channels and groups that would be left without changing
your account. To leave all listed chats, explicitly confirm the operation:

```sh
go run . --confirm
```

On first use, enter the verification code sent by Telegram at the terminal.
The authenticated session is stored under `.tdlib/` and reused by later runs.
Use `--config path/to/config.yaml` to load a different configuration file.

On macOS, if TDLib is installed outside the standard paths, supply its include
and library paths:

```sh
CGO_CFLAGS="-I/path/to/tdlib/include" \
CGO_LDFLAGS="-Wl,-rpath,/path/to/tdlib/lib -L/path/to/tdlib/lib -ltdjson" \
go run . --confirm
```
