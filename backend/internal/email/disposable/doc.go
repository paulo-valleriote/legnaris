// Package disposable refuses addresses whose domain cannot receive a
// verification code: a known throwaway provider from the embedded blocklist, or
// a domain that answers the MX lookup with no mail exchange.
package disposable
