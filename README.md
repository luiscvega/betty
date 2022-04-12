# betty

This is a simple program that will fetch the latest records (limit 500) and create records for these. If new records are detected in subsequent fetches, it will notify those to the Slack hook. Currently, I have disabled the notifying the for now.

It will get an access token first then call the actual transactions API call to UB using the access token.

```go
go build -o betty
./betty
```
