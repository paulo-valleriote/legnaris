# Contract: Server state

**Requirements**: WCT-02, WCT-03, WCT-07, WCT-08, WCT-12, WCT-16, WCT-21 | **Backing document**:
[research.md R1…R4, R9](../research.md)

The catalog and the template detail are server state owned by one TanStack Query client. This contract
is what the query module (`web/src/catalog/queries.ts`) implements and what the screen suites assert
through the injected transport.

## The provider seam

| Element | Rule |
|---|---|
| `AppProviders` (`web/src/app/providers.tsx`) | builds one `ApiClient` over the F2 fetch transport and the resolved base address, creates one `QueryClient`, and renders both providers around the app |
| client access | the query hooks read the client from React context (`useApiClient`), never from a module-level singleton |
| test access | the render helper (`web/src/test/render.tsx`) mounts the same providers with an injected client and a fresh query client per test |
| query client defaults | `retry: false`, so no query falls back to the library's built-in retry |

## Query configuration

| Query | Key | `staleTime` | `gcTime` | Other | Obligation |
|---|---|---|---|---|---|
| catalog | `['templates']` | `Infinity` | default | `retry: false`, `refetchOnWindowFocus: false`, `refetchOnMount: false`, `refetchOnReconnect: false` | exactly one request per app load (WCT-02); no background, focus or reconnect request |
| detail | `['template', id]` | `0` | `0` | `retry: false`, `refetchOnWindowFocus: false` | exactly one request per opening (WCT-21); no reuse across openings (WCT-13) |

## Obligations

1. A first mount of the catalog screen issues exactly one request (WCT-02); remounting, refocusing the
   window or reconnecting issues none.
2. A trigger while the query for a key is in flight joins the in-flight request; no second concurrent
   request is started (WCT-08).
3. Activating Retry while the query is `error`/`parseError` issues exactly one further request
   (WCT-07); activating it again while that request is in flight issues none (WCT-08).
4. Opening the surface issues exactly one detail request for that identifier (WCT-12); opening the same
   identifier again issues exactly one further request and shows `loading` first (WCT-21, WCT-13).
5. No query issues a mutating request: both operations are reads, and the surface's primary action
   calls no query function (WCT-25).
6. `retry: false` on the query client means the retry policy is the F2 client's alone — a read that
   fails at the transport is repeated once by the client, never by the query layer.

## Test obligations

- The suite injects a fake transport and asserts the number of transport attempts per scenario: one per
  app load, one per opening, one per Retry, and no second while in flight.
- A case remounts the catalog screen (and fires a window focus event) and asserts the attempt count is
  unchanged.
- A case reopens the same template and asserts the attempt count increased by exactly one and that the
  loading state was rendered before the result.
