# tg-cleaner

Leave Telegram channels and groups where the account isn't an administrator,
while retaining private conversations and administered chats.

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
go run . leave
```

The preview lists non-admin channels and groups that would be left without
changing your account. Chats owned or administered by the account are always
excluded. To leave all listed chats, explicitly confirm the operation:

```sh
go run . leave --confirm
```

For rate-limit safety, the command waits a randomized 10–15 seconds between
leave requests. If Telegram returns a rate-limit response, it honors the
requested wait before retrying. Press Ctrl+C to stop safely at any time.

On first use, enter the verification code sent by Telegram at the terminal.
The authenticated session is stored under `.tdlib/` and reused by later runs.
Use `--config path/to/config.yaml` to load a different configuration file.

On macOS, if TDLib is installed outside the standard paths, supply its include
and library paths:

```sh
CGO_CFLAGS="-I/path/to/tdlib/include" \
CGO_LDFLAGS="-Wl,-rpath,/path/to/tdlib/lib -L/path/to/tdlib/lib -ltdjson" \
go run . leave --confirm
```
