Coverage policy and how-to
=========================

Zweck
-----
Dieses Dokument beschreibt, wie Coverage lokal erzeugt wird und wie die CI den Coverage-Status prüft.

Grundsätze
----------
- Coverage-Ziel ist per Default 100% (COVERAGE_TARGET), kann aber in der CI oder lokal gesetzt werden.
- Generierte Mocks oder Test-only Pakete können bei Bedarf ausgeklammert (nicht empfohlen) werden.

Lokal: schnelle Befehle
----------------------
Erzeuge Coverage-Report und HTML:

```bash
make coverage
# oder (direkt)
go test ./... -coverprofile=coverage.out -covermode=atomic -timeout 60s
go tool cover -html=coverage.out -o coverage.html
```

Coverage-Check (erzeugt Fehler, wenn die Abdeckung unter dem Ziel liegt):

```bash
make coverage-check COVERAGE_TARGET=100
```

CI

Eine GitHub Actions Workflow-Datei `.github/workflows/coverage.yml` führt `make coverage-check` aus und lädt `coverage.out` und `coverage.html` als Artefakte hoch. Der Workflow setzt standardmäßig `COVERAGE_TARGET=100`.

Tipps
-----
- Wenn du generierte Mocks nicht mitrechnen willst, kannst du die `go test` Kommandozeile anpassen und Pakete ausschließen (z. B. `go list ./... | grep -v test | xargs go test ...`).
- Bei großen Refactorings empfehlen wir schrittweise Erhöhung der Coverage (z. B. 80% -> 90% -> 100%).

Kontakt
-------
Bei Fragen zum Coverage-Workflow: öffne ein Issue oder schreibe in den Repo-PR mit dem Tag `coverage`.
