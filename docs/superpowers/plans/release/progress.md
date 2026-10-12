# Release execution ledger — 2026-10-12-release.md

Starting commit: 2e23d64. Foundation CI 38213317304 passed on Windows/macOS/Linux and browser; first foundation is not the finished app.
User authorizes a complete plan and step-by-step implementation/testing. Execute inline; approved spec remains binding.
Pre-flight: downloads Importer consumes rooted library.Import; desktop manager owns profile HTTP servers and queues; React browser API keeps existing authentication; native administration is private RPC. Interfaces checked.
Current task: 1, persistent downloads. Remaining tasks: conversion/artwork/settings; keyring/mobile protocol; desktop manager; React; Tauri/installers; release validation.
Release constraints: no signing secrets or outbound identities configured; real iOS/Android device tests cannot be inferred from CI. Build all implementation and development artifacts first, then identify concrete external verification gates.
Task 1 progress: download queue tests RED→GREEN for persistence, cancellation/interruption, retry, shared-library collisions, outside-staging rejection, playlist expansion into individual jobs, and bounded real subprocess output. Authenticated queue API tests RED→GREEN. yt-dlp 2026.08.19 downloaded and verified against published SHA-256, version command PASS. GTK/WebKit build dependencies installed inside a dedicated development container because the host user cannot install system packages; packaged app will not require Docker.
