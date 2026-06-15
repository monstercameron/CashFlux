module github.com/monstercameron/CashFlux

go 1.26.0

require github.com/monstercameron/GoWebComponents v0.0.0

// CashFlux is built directly on top of the local GoWebComponents checkout.
// These replaces mirror the directives the framework declares for its own
// local-only modules so the dependency resolves without a published proxy copy.
replace github.com/monstercameron/GoWebComponents => ../GoWebComponents

replace agenthub => ../GoWebComponents/tools/agenthub

replace github.com/monstercameron/GoGRPCBridge => ../GoWebComponents/third_party/GoGRPCBridge
