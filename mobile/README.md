# mobile

Flutter app for scale-app's vendor-facing mobile client. Not yet built —
this directory is a placeholder in the monorepo layout.

## Planned screens

- **Vendor login** and **admin login** (admin can add/remove vendor users).
- **Connection/scales overview**: connection status and all scales
  available on the network.
- **Transaction flow**: pick a scale → pick a product (with its price from
  the product DB) → send the price to the scale → the scale weighs, computes
  the total, and shows it to the customer on its own display → the app reads
  back the approved transaction, the vendor verifies weight/price/total, and
  locks it into the current receipt with a tap.
- **Mutable draft receipt**: the in-progress receipt can be edited at any
  point before it's finalized, so a vendor can correct a mistake.

Exact UI/UX design to follow; this will be scaffolded once that's
available, and will talk to the `scale-gateway` service's HTTP API (see
`../backend/services/scale-gateway/README.md`) for scale interactions.
