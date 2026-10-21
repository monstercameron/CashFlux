# Server AI cost accounting

When Cloud sync is connected and AI uses the server, the browser sends an authenticated `AIService` gRPC request. The server uses that account's encrypted-at-rest OpenAI key to call the provider. Prompts and responses are needed transiently for the call; CashFlux does not put them in the usage table or cost log.

Schema v22 records daily request count, billed input/output token total, and an estimated cost in nano-USD per account. It retains only the daily aggregate. Successful calls, streamed calls, and provider responses that report usage but produce no usable answer are counted. Calls without a known model price mark the daily estimate incomplete. On migration, earlier token usage is also marked incomplete because its model and price cannot be recovered.

The operator console's account detail and usage history show these estimates. The server process log records model, token counts, estimated cost, and whether a model price was known; it omits account identity, prompt text, answer text, and provider keys from the new cost event.

Estimates use standard input and output prices checked on 2026-10-21 against [OpenAI's API pricing](https://developers.openai.com/api/docs/pricing). They are not invoices. Cached input, long contexts, regional processing, provider price changes, and other processing tiers can change the actual charge. Compare the provider bill for authoritative spending.
