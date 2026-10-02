# Admin efficiency extension

Managed since Stage30 (2026-10-02). This is a compiled-in extension with runtime
write admission, not a loadable binary plugin. Its catalog ID is `admin-efficiency`.

## Ownership and ports

- Upstream-group types, normalization and directory operations: independent reader,
  renamer and sorter ports; host repository retains atomic label propagation.
- Display sorting: `AccountSortOrderRepository`; no scheduling-priority changes.
- Membership replacement: `GroupAccountPorts[T]`; host validates account existence,
  platform and OAuth-only constraints, and retains atomic persistence.
- Batch users: positive IDs, max 500, ordered deduplication and per-user results.
  Host callbacks retain admin protection, authorization and deletion cleanup.
- `StateReader` is supplied by the host settings adapter. No imports of the host
  service/repository or generated Ent models are permitted.

## Admission

Host sorting, directory mutations and user batch handlers call `CheckWrite` before
mutation; membership replacement checks it inside its orchestration. New/changed
upstream labels use `CheckUpstreamGroupChange`, including explicit clearing and
inherited shadow labels. Native duplicate behavior does not copy upstream labels.
Unknown/invalid/unavailable state fails closed. Exact unchanged labels are allowed
so native account edits can preserve history.

Reads never use write admission: account lists, historical directory labels, order
and group membership remain readable. Native account/user/group CRUD and upstream
native group sorting are not blanket-gated. Disabling never deletes settings or data.

## Frontend and installation

`frontend/src/extensions/modules/admin-efficiency` owns APIs and UI; host facades
preserve old imports and inject native account ports. The host composable supplies
the flag; no blanket route gate is used. Writes are checked again after dialogs or
confirmation; native creation/editing omits disabled custom fields. Bulk user delete
uses the batch endpoint, never a loop over native single-delete.

The separate one-time adoption migration enables recognized legacy installations,
disables native installs and preserves existing choices. Its ledger and setting are
atomic; the runner's earlier successful independent migrations remain committed if
a later migration fails. No live database or deployment is implied by unit fixtures.

## Verification boundary

Module, host, migration and frontend tests cover admission and history, not real
PostgreSQL concurrency, browser interaction with the deployed process, or upgrades
against an unexamined future upstream version. These remain release checks.
