# Media gateway extension

`media-gateway` owns protocol endpoint registration and admission policy. Host adapters keep authentication, scheduling, task ownership, persistence, billing and refunds; this is a compiled extension with a runtime switch, not a dynamically unloaded driver.

## Admission and drain contract

- Enabled: accept new Grok images, video create/edit/extend, Seedance, Gemini Veo predictLongRunning, voice synthesis/recognition/custom-voice creation and new realtime voice sessions.
- Disabled or missing flag: reject new work before concurrency slots/account selection. Invalid/unavailable settings fail closed.
- Grok Responses HTTP and WebSocket image_generation tools pass the same admission; passive client image_gen namespaces, client function tools, text and vision inputs are not media submissions.
- Existing task status, downloads, cancellation and settlement remain available without reading the switch. Ownership/authentication checks still run. Already admitted work and open realtime sessions drain normally; subsequent new WebSocket generation turns require new admission.
- Native text/token/model APIs and native OpenAI image endpoints are not owned by this switch. Playground/infinite-canvas have their own ingress admission, rather than double-gating their retained tasks here.

`endpoint.go`, `admission.go` and `responses.go` import neither host service/repository/handler packages nor Ent. Host adapters remain the integration seam. There is deliberately no frontend route gate that would hide historical tasks.

## Adoption and tests

The standalone ledger migration adopts legacy installs as enabled and native installs as disabled, preserving explicit values and not reseeding deleted flags. Unit tests cover the state matrix, real new-work rejection before scheduling, protocol classifiers and retained ownership checks. SQLite migration tests and local builds are not PostgreSQL migration rehearsal or deployed/browser acceptance.
