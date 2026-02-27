# Test-Helpers für Provider-Tests

Diese Datei beschreibt die Test-Hilfsobjekte unter `anx/provider/test/`, insbesondere die in-memory FakeAPI, das Verhalten von `FakeExisting` / `Existing` sowie Hinweise zur Mock-Generierung mit `mockgen`.

## FakeAPI (Kurzüberblick)

Datei: `anx/provider/test/fake_api.go`

FakeAPI ist ein leichter In-Memory-Fake, der die generierten GoMock-Mocks (z. B. `apimock.MockAPI`) um eine Zustandsspeicherung ergänzt. Er wird von Tests verwendet, um realistische API-Antworten zu simulieren, ohne externe Abhängigkeiten.

Wesentliche Eigenschaften:

- Hält eine interne `store map[string]types.Object` und `tags map[string][]string`.
- Bietet `FakeExisting(o types.Object, tag ...string) string` zum Vorbefüllen des Stores und zur gleichzeitigen Vergabe von Tags.
- `Existing()` liefert kompatible Objekte für die bestehenden Gomega-Matcher zurück (siehe Hinweis weiter unten).
- Implementiert `Create`, `Get`, `Update`, `Destroy`, `List` als Proxy-Methoden (werden auf die Fake-Implementierungen `doCreate`, `doGet`, ... gemappt).
- Optionaler `SetPreCreateHook`-Hook, den Tests setzen können, um erzeugte Objekte vor dem Speichern anzupassen.

## Wichtige Methoden und Semantik

### FakeExisting

- Speichert eine (tiefe) Kopie des übergebenen `types.Object` in `store`.
- Wenn das Objekt keine Identifier hat, wird eine zufällige Identifier-String gesetzt (sofern das Feld existiert und schreibbar ist).
- Optional können Tags mitgegeben werden; diese werden dedupliziert im `tags`-Mapping hinterlegt.
- Bei LBaaS-Ressourcen werden interne `typeMap`-IDs gesetzt, damit `Resource.Type.Identifier` beim Listen geliefert werden kann.

Rückgabewert: die verwendete Identifier-String.

### Existing

- Liefert eine Slice von `*mock.APIObject` (`go.anx.io/go-anxcloud/pkg/api/mock.APIObject`) zurück, nicht lediglich rohe `types.Object`.
- Diese Wrapper sind kompatibel mit den vorhandenen go-anxcloud-Gomega-Matchern (z. B. `matcher.Object(...)`), die zur Laufzeit auf genau diesen Typ prüfen.
- Implementation: Die FakeAPI erzeugt temporär ein echtes `mock`-API, ruft `FakeExisting` für die tiefen Kopien auf und gibt die `*mock.APIObject`-Pointer über `Inspect` zurück.

Hinweis: Falls du die Methode anders verwendest, achte darauf, dass die Matcher in go-anxcloud `*mock.APIObject` erwarten (siehe `go.anx.io/go-anxcloud/pkg/api/mock/matcher`).

### List / ObjectChannel

- `List` unterstützt das Setzen von `types.ListOptions{ObjectChannel: &c}`.
- FakeAPI snapshotet `store` und `tags` unter Mutex und sendet dann asynchron `types.ObjectRetriever`-Closures in den Channel.
- Wenn ein `FilterObject` `Tags` enthält, werden nur Objekte mit diesen Tags zurückgegeben.
- Die Retrievers setzen mindestens `Identifier` und, falls bekannt, `Resource.Type.Identifier` (aus `typeMap`) auf dem übergebenen Objekt.

Wichtig: Die Closures kopieren oder setzen Felder per Reflection; die meisten Provider-Tests erwarten, dass `Identifier` und `Type.Identifier` vorhanden sind.

### ResourceWithTag

- `Create` behandelt `*corev1.ResourceWithTag` speziell: die `Identifier` dort ist eine Referenz auf ein Zielobjekt. Beim Aufruf wird die ID nur zu `tags[tag]` hinzugefügt, wenn das Ziel bereits im Store existiert. Dadurch werden ungewollte Tag-Eintrags-Wachstümer vermieden.

### Hooks

- `SetPreCreateHook(func(ctx context.Context, a api.API, o types.Object))` erlaubt Tests, generierte Objekte (z. B. `Server`) zu modifizieren, bevor sie in den Store geschrieben werden (z. B. den `State` setzen).

## Mock-Generierung (mockgen)

Die Repository-Policy ist: GoMock (mockgen) statt mockery/testify.

Beispiele zur Regenerierung der wichtigsten Mocks (aus `anx/provider/test/utils.go`):

```sh
# z.B. zentrale API-Mock
mockgen -package gomockapi -destination ./anx/provider/test/gomockapi/api.go go.anx.io/go-anxcloud/pkg API

# LBaaS-spezifische Mocks
mockgen -package mocklbaas -destination ./anx/provider/test/mocklbaas/lbaas.go go.anx.io/go-anxcloud/pkg/lbaas API
mockgen -package mocklbaas -destination ./anx/provider/test/mocklbaas/backend/backend.go go.anx.io/go-anxcloud/pkg/lbaas/backend API
# ... analog für bind, server, frontend, loadbalancer
```

Hinweis: Die generierten GoMock-Dateien liegen im Repo unter `anx/provider/test/gomockapi`, `anx/provider/test/mocklbaas`, `anx/provider/test/mockvsphere`, etc.

## Beispiel: Minimaler Test-Setup

```go
ctrl := gomock.NewController(t)
api := test.NewFakeAPI(ctrl)
// optional: api.SetPreCreateHook(...)
// pre-fill objects
lbID := api.FakeExisting(&lbaasv1.LoadBalancer{Identifier: "lb-1"})

// bei Bedarf: abrufen
objs := api.Existing() // liefert []*mock.APIObject
```

Beim Verwenden von `objs` in Gomega-Assertions (z. B. `Expect(objs).To(ConsistOf(Object(...)))`) sind die `*mock.APIObject`-Wrapper korrekt für die vorhandenen go-anxcloud-Matcher.

## Tests lokal ausführen

Empfohlenes Kommando zum lokales Testen (wie im Makefile / CONTRIBUTING):

```sh
go test ./... -v -timeout 60s
```

## Änderungen / Wartung

- Falls sich die Matcher-Implementierung in `go-anx.io/go-anxcloud` ändert, muss die `Existing()`-Strategie ggf. angepasst werden.
- Wenn neue API-Interfaces hinzukommen, generiere die GoMock-Mocks mit `mockgen` und lege sie unter `anx/provider/test/` ab.

---

Bei Fragen oder wenn du ein Beispiel für einen konkreten Test brauchst, schreibe kurz, ich ergänze eine Muster-Testdatei unter `anx/provider/test/examples`.
