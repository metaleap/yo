//go:build debug

package yopenapi

import (
	"yo/util/str"
)

var Description_IntroNotes = str.Replace(str.Trim(`
This HTTP API has RPC rather than REST semantics: **all** operations are ´POST´, regardless of what CRUD writes or reads they might or might not effect.

**tl;dr:** **usually, API requests will just-work as expected _without_ knowing all those intro notes right below** (which elaborate mostly to-be-expected software-dev-commonplaces) — but in any cases of unexpected results or errors, they'll likely help complete the mental picture.
___
Our backend stack's "opinionated convention-over-configuration" designs yield a few request/response rules that predictably remain **always in effect across all listed operations**:
- Whereas request and response bodies are operation-specific, all operations share the exact-same set of request headers, URL query-string parameters and response headers (albeit being elaborated here identically and redundantly for each individual operation).
- The empty request body is principally the JSON ´{}´, but fully-empty or JSON ´null´ request bodies are interpreted equivalently.
- Response bodies will never be empty, but may be the JSON ´null´.
- Request and response bodies are always valid JSON values for _JSON objects_, ie. they're never immediately JSON arrays, ´string´s, ´number´s, or ´boolean´s.
- All mentioned request-object (and sub-object) fields are **by default optional** and ommittable or ´null´able (implying for atomic types semantic equivalence to ´""´ or ´0´ or ´false´ as per _Golang_ type-system semantics),
  - **any exceptions** to this optionality-by-default are indicated by the operation's listed known-error responses.
- All mentioned response-object (and sub-object) fields will always be present in the response-body, indicating their default-ness / missing-ness via ´null´ or ´""´ or ´0´ or ´false´ as per _Golang_ type-system semantics.
  - Caution for some client languages: this means ´null´ for some-but-not-all empty JSON arrays (with ´[]´ being principally always just-as-possible) and empty JSON dictionary/hash-map "object"s (with either ´null´ or ´{}´ being principally equally possible).
- All JSON (non-dictionary/non-hash-map) object field names begin with an upper-case character,
  - any operation-specific example rendered to the contrary indicates a "free-style" JSON dictionary/hash-map "object".
- The ´Content-Length´ request header is **required for all** operations (with a correct value).
- The ´Content-Type´ request header is optional, but if present, must be correct with regards to both the operation's specification and the request body.
- Any ´{ctype_multipart}´ operations:
  - **always require** the following two form-fields, ignoring all others: ´files´ for any binary file uploads, and ´_´ for the actual JSON request payload;
  - only the ´_´ field value is further elaborated for any such operation in this doc, and always in the exact same way as also done in this doc for all the ´{ctype_json}´ operations' request bodies (**without** specifically mentioning the ´_´ form-field containing the ´{ctype_text}´ of the full ´{ctype_json}´ request payload actually being elaborated there).

How to read request/response **example JSON values** rendered in this doc:
  - ´true´ indicates _any_ ´boolean´ value, regardless of the actual real value in a call;
  - ´"someStr"´ indicates _any_ ´string´ value;
  - signed-integer ´number´s are indicated by a negative-number example indicating the minimum (type-wise, not operation-specific) permissible value, with the maximum being the corresponding positive-number counterpart;
  - unsigned-integer ´number´s are indicated by a positive-number example indicating the maximum (type-wise, not not operation-specific) permissible value, with the minimum being ´0´;
  - floating-point ´number´s are indicated by a positive-number example indicating the maximum (type-wise, not not operation-specific) permissible value, with the minimum being the corresponding negative-number counterpart.
  - date-time values are indicated by RFC3339/ISO8601-formatted ´string´ examples:
    - in responses, they're always UTC, whereas in requests, any timezone may be indicated;
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
  - ´200´ with response-body of usually JSON ´null´ (exception: an operation-specific response object with a field name obviously indicating not-found-ness) — for operations where the definite-existence expectation does not hold as crucially (for example those of the "fetch single/first object found for some specified criteria" kind).

`), str.Dict{"´": "`"})

var Description_MultipartNotes = str.Replace(str.Trim(`´{ctype_multipart}´ form-fields:
1. binary file uploads in form field ´files´, and
2. ´{ctype_json}´ request payload as ´{ctype_text}´ in form field ´_´, as elaborated in this example ({type_ident_hint})
`), str.Dict{"´": "`"})
