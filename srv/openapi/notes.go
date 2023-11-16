//go:build debug

package yopenapi

import (
	"yo/util/str"
)

var Description_IntroNotes = str.Replace(str.Trim(`
This HTTP API has RPC-ish rather than REST semantics: **all** operations are ´POST´, regardless of what CRUD writes or reads they might or might not effect.

<small>(For JS/TS clients, there's a better-than-generated-from-_openapi.json_ clienting package (fully deps-free) at ´/__yostatic/yo-sdk.js´ and ´/__yostatic/yo-sdk.ts´. They're always in sync with this _openapi.json_.)</small>

**tl;dr:** **usually, API requests will just-work as expected _without_ knowing all those intro notes right below** (which elaborate mostly to-be-expected software-dev-commonplaces) — but in any cases of unexpected results or errors, they'll likely help complete the mental picture.
___
Our backend stack's (named "yo") "opinionated convention-over-configuration" designs yield a few request/response rules that predictably remain **always in effect across all listed operations**:
- Whereas request and response bodies are operation-specific, all operations share the exact-same set of request headers, URL query-string parameters and response headers.
- The empty request body is principally the JSON ´{}´, but fully-empty or JSON ´null´ request bodies are permissible and interpreted as ´{}´.
  - Response bodies are never empty and are never the JSON ´null´.
- Request and response bodies are always valid JSON values for _JSON objects_, ie. they're never immediately JSON arrays, ´string´s, ´number´s, or ´boolean´s.
  - They're also never immediately "domain objects" (ie. a ´GetFoo´ op would yield not a ´Foo´ but a ´Result´ (or similar) field with the found ´Foo´; or a ´CreateFoo´ would expect not a ´Foo´ payload but a ´NewFoo´ (or similar) field with the new ´Foo´).
- All mentioned request-object/sub-object fields are **by default optional** and omittable or ´null´able (implying for atomic types semantic equivalence to ´""´ or ´0´ or ´false´ as per _Golang_ type-system semantics),
  - **any exceptions** to this optionality-by-default are indicated by the operation's listed known-error responses.
- All mentioned response-object/sub-object fields will always be present in every response-body, indicating their default-ness / missing-ness / empty-ness via ´null´ or ´""´ or ´0´ or ´false´ as per _Golang_ type-system semantics;
  - empty/unset JSON arrays are never ´null´ but ´[]´; empty JSON dictionary/hash-map objects are never ´{}´ but always ´null´.
- All (non-dictionary/non-hash-map) JSON object field names known to the backend begin with an upper-case character,
  - any operation-specific examples of JSON objects with lower-case-beginning keys/fields indicate a JSON dictionary/hash-map object.
- The ´Content-Length´ request header is **required for all** operations (with a correct value).
- The ´Content-Type´ request header is optional, but if present, must be correct with regards to both the operation's specification and the request body.
- Any ´{ctype_multipart}´ operations:
  - **always require** the following two form-fields, ignoring all others: ´files´ for any binary file uploads, and ´_´ for the actual JSON request payload;
  - only the ´_´ field value is further elaborated for any such operation in this doc, and always in the exact same way as also done in this doc for all the ´{ctype_json}´ operations' request bodies (**without** specifically mentioning the ´_´ form-field containing the ´{ctype_text}´ of the full ´{ctype_json}´ request payload actually being elaborated there).
- Create operations of DB-stored objects (those with ´Id´ and ´DtMade´ and ´DtMod´ fields) ignore those very fields, they best be omitted in the client-side call site for clarity;
  - Update operations of such objects likewise ignore those very fields, usually offering a separate field to clearly identify the object(s) requested to be modified.

How to read request/response **example JSON values** rendered in this doc:
  - ´true´ indicates _any_ ´boolean´ value, regardless of the actual real value in a call;
  - ´"someStr"´ indicates _any_ ´string´ value;
  - **signed-integer** ´number´s are indicated by a negative-number example indicating the minimum (type-wise, not operation-specific) permissible value, with the maximum being the corresponding positive-number counterpart;
  - **unsigned-integer** ´number´s are indicated by a positive-number example indicating the maximum (type-wise, not operation-specific) permissible value, with the minimum being ´0´;
  - **floating-point** ´number´s are indicated by a positive-number example indicating the maximum (type-wise, not operation-specific) permissible value, with the minimum being the corresponding negative-number counterpart.
  - **date-time values** are indicated by RFC3339/ISO8601-formatted ´string´ examples:
    - in responses, they're always in **UTC**, whereas in requests, any timezone may be indicated;
	- in requests, they may always be ´null´ (excepting any operation-specific known-errors indicating otherwise) but must never be ´""´ or otherwise non-RFC3339/ISO8601-parseable.

About **error responses**:
- All are ´{ctype_text}´.
- In addition to those listed in this doc (thrown by the service under the indicated conditions), other error responses are at all times entirely technically-possible and not exhaustively documentable (feasibly), such as eg. DB / file-system / network disruptions. Those caught by the service will be ´500´s, others (ie. from load-balancers / gateways / reverse-proxies etc. _in front of_ the service) might use _any_ HTTP status code whatsoever.
- All the well-known (thrown rather than caught) errors listed here:
  - have their code-identifier-compatible (spaceless ASCII) enumerant-name as their entire text response, making all error responses inherently ´switch/case´able;
  - have been recursively determined by code-path walking. Among them are some that logically could not possibly ever occur for that operation, yet identifying those (to filter them out of the listing) is (so far) out of scope for our current ´{spec_file_name}´ spec generator. (In case of serious need, do let us know!)
- Any non-known (caught rather than thrown) errors (not listed here) contain their original (usually human-language) error message fully, corresponding to the ´default´ in an error-handling ´switch/case´.
- **"Not Found" rules:**
  - ´404´ **only** for HTTP requests with definitely-unroutable URL paths (ie. "no such API operation or static-file asset or sub-site or etc."),
  - ´406´ with approx. ´FooDoesNotExist´ (see per-operation known-error-responses listings for exact monikers) — for operations where existence was definitely critically expected (such as modifying some object identified by its ´Id´),
  - ´200´ with some operation-specific response-body field (like ´Result´ or similar) being ´null´ — for operations where the definite-existence expectation does not hold as crucially (for example those of the "fetch single/first object found for some specified criteria" kind).

`), str.Dict{"´": "`"})

var Description_MultipartNotes = str.Replace(str.Trim(`´{ctype_multipart}´ form-fields:
1. ´files´ for any binary file uploads, and
2. ´_´ for the actual ´{ctype_json}´ request payload, as elaborated in this example ({type_ident_hint})
`), str.Dict{"´": "`"})
