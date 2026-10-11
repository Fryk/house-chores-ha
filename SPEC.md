# Obowiązki — app (add-on) do Home Assistant

Specyfikacja dla Claude Code. Wersja 0.5, stan na 2026-10-11.

Repozytorium: https://github.com/Fryk/house-chores-ha

Oznaczenia wiarygodności faktów technicznych:
- **[docs]** — sprawdzone 2026-10-11 w oficjalnej dokumentacji HA (linki na końcu)
- **[trening]** — z wiedzy modelu, niezweryfikowane. Przed implementacją sprawdź w dokumentacji i popraw spec, jeśli się nie zgadza.

W wersji 0.5 wszystkie punkty [trening] zostały sprawdzone w źródłach dokumentacji (pkt 12). Zostały tylko dwa, których dokumentacja HA nie opisuje: dostęp do prywatnego repo (pkt 9.1) i schowek w WebView (pkt 8.1); ścieżkę panelu dla `clickAction` trzeba odczytać w M0 (pkt 5).

---

## 1. Cel

Prosty system domowych obowiązków dla kilku domowników. Jedyny kanał przypomnień to powiadomienia Android wysyłane przez Home Assistant Companion App. UI działa wewnątrz aplikacji HA (ingress), bez osobnej aplikacji mobilnej i bez osobnego logowania. Appka czyta stany urządzeń z HA (pralka, suszarka, zmywarka), żeby sama zakładać zadania.

Poza zakresem: listy zakupów, nagrody i odznaki, tworzenie encji lub list to-do w HA, inne kanały powiadomień, iOS (nie blokować, ale nie testować).

## 2. Typy zadań

Globalne okno powiadomień: **10:00–20:00**. Po 20:00 nic nie jest wysyłane.

### 2.1 Zadanie na konkretny dzień (`dated`)
Ma termin dzienny. Warianty powtarzalności:
- jednorazowe (data)
- w wybrane dni tygodnia (np. każdy czwartek)
- co N dni / tygodni od daty startowej
- N dni po ostatnim wykonaniu

Przypomnienia: w dniu terminu od 10:00, ponawiane co `repeat_interval` (domyślnie 3 h) do odhaczenia, w ramach okna. Każde zadanie ma własne powiadomienie z przyciskami.

Niewykonane w terminie: zostaje otwarte jako zaległe i przypomina się kolejnego dnia od 10:00. Gdy nadejdzie następne wystąpienie tego samego zadania, stare zamyka się ze statusem `missed`.

### 2.2 Zadanie tygodniowe (`weekly`)
Do zrobienia w dowolnym momencie tygodnia (pon.–niedz.). Przypomnienia **narastająco**, domyślny harmonogram (konfigurowalny globalnie):

| Dzień | Godziny |
|---|---|
| poniedziałek | 10:00 |
| czwartek | 10:00 |
| sobota | 10:00 |
| niedziela | 10:00, 16:00 |

Przypomnienie tygodniowe to **jedno zbiorcze powiadomienie** na osobę („Zostało w tym tygodniu: odkurzanie, łazienka, pranie"), a nie osobne na każde zadanie. Tapnięcie otwiera listę w appce. Powód: limit 3 przycisków na Androidzie i unikanie spamu.

Niewykonane do końca niedzieli: status `missed`, w poniedziałek startuje nowe wystąpienie.

### 2.3 Zadanie bez terminu (`adhoc`), np. śmieci
Nie ma harmonogramu. Każdy sam wchodzi do appki i ma przy zadaniu dwa przyciski:

- **„Trzeba zrobić"** — tworzy otwarte wystąpienie i wysyła powiadomienie do wszystkich, z przyciskami. Ponawiane co `repeat_interval` w oknie do odhaczenia. Zgłoszenie poza oknem czeka z pierwszym powiadomieniem do 10:00.
- **„Zrobione"** — od razu zapisuje wykonanie (bez wcześniejszego zgłoszenia) i wysyła do wszystkich powiadomienie informacyjne „Paweł: wyniesienie śmieci ✅".

Odhaczenie zgłoszonego zadania (z appki lub z przycisku w powiadomieniu) również wysyła powiadomienie informacyjne i kasuje wiszące przypomnienie na pozostałych telefonach.

Na kafelku zadania widać: kto i kiedy zrobił ostatnio.

### 2.4 Zadanie wyzwalane z HA (`triggered`)
Appka obserwuje wskazane encje HA i sama reaguje na cykl urządzenia. Dwa tryby reakcji:

- **`ask_who`** — zdarzenie już się stało, pytamy kto to zrobił. Powiadomienie do wszystkich: „Pralka ruszyła. Kto włączył?" z przyciskiem „Ja". Pierwsze kliknięcie zapisuje wykonanie na tę osobę i kasuje pytanie u pozostałych. Bez ponawiania. Brak odpowiedzi: wpis trafia do sekcji „Nieprzypisane" w appce, gdzie można go przypisać ręcznie.
- **`needs_doing`** — zdarzenie tworzy zadanie do zrobienia. Działa jak „Trzeba zrobić" z pkt 2.3: powiadomienie do wszystkich z przyciskami „Zrobione" i „Za godzinę", ponawiane w oknie do odhaczenia.

Konfiguracja startowa:

| Urządzenie | Start cyklu | Koniec cyklu |
|---|---|---|
| Pralka | `ask_who`: „Wstawienie prania" | `needs_doing`: „Wyjęcie prania" |
| Suszarka | nie śledzimy | `needs_doing`: „Wyjęcie z suszarki" |
| Zmywarka | nie śledzimy | `needs_doing`: „Rozładowanie zmywarki" |

Wkładanie i uruchamianie suszarki oraz zmywarki celowo nie jest liczone.

Wykrywanie cyklu, do wyboru per urządzenie w ustawieniach:
- **stan encji** — lista stanów oznaczających „pracuje" i „skończone" (np. `run` → `stop`)
- **pobór mocy** — encja liczbowa: powyżej progu X W przez N min = pracuje, poniżej progu Y W przez M min = skończone

Wymagany debounce (krótkie skoki stanu nie tworzą zadań) i automat stanów per urządzenie: `idle → running → finished → idle`. Jeśli ruszy nowy cykl, a poprzednie „wyjęcie" jest wciąż otwarte, stare zamyka się jako wykonane bez osoby („Nieprzypisane").

Koniec cyklu po 20:00: zadanie powstaje od razu i jest widoczne w appce, a pierwsze powiadomienie idzie o 10:00.

Technicznie: subskrypcja zmian stanu przez WebSocket **[docs]**. `subscribe_events` przyjmuje tylko opcjonalne `event_type`, **bez filtra po encjach** — przy `state_changed` appka dostaje zmiany wszystkich encji i sama filtruje po `entity_id`. Alternatywa: `subscribe_trigger` z triggerem `platform: state` i listą `entity_id` (składnia jak w automatyzacjach); serwer odsyła wtedy wiadomości typu `trigger`. Lista encji do wyboru z `GET /api/states` (tablica obiektów `entity_id`, `state`, `last_changed`, `attributes`) albo z komendy WS `get_states` **[docs]**.

## 3. Katalog obowiązków

Przy pierwszym uruchomieniu appka pokazuje **predefiniowaną listę** typowych obowiązków do zaznaczenia. Zaznaczone stają się aktywnymi zadaniami z domyślnym typem, który można zmienić. Własne zadania dodaje się w dowolnym momencie, katalog jest dostępny też później.

Zestaw startowy:

- **Bez terminu:** wyniesienie śmieci (zmieszane, bio, plastik, papier, szkło jako osobne pozycje), zakupy spożywcze, rozładowanie zmywarki, wstawienie prania, wyjęcie prania, wyjęcie z suszarki, składanie prania
- **Tygodniowe:** odkurzanie, mycie podłóg, sprzątanie łazienki, sprzątanie toalety, sprzątanie kuchni, wycieranie kurzy, zmiana pościeli, pranie ręczników, podlewanie kwiatów
- **Na dzień / okresowe:** wystawienie pojemników na śmieci, mycie okien, mycie lodówki, czyszczenie filtrów (okap, odkurzacz, wentylacja), odkamienianie ekspresu, koszenie trawnika

Pozycje związane z pralką, suszarką i zmywarką po skonfigurowaniu wyzwalaczy przechodzą na typ `triggered`, ale nadal da się je odhaczyć ręcznie.

## 4. Osoby i odbiorcy

- **Osoba**: nazwa, kolor (do kalendarza i wykresów), `ha_user_id`, jedna lub więcej usług `notify.mobile_app_*`.
- **Lista odbiorców**: nazwana, zapisana grupa osób. Wbudowana lista „Wszyscy". Appka trzyma listy, żeby nie wypełniać ich przy każdym zadaniu.
- Zadania `dated` i `weekly`: odbiorcą jest „Wszyscy" albo wybrana lista.
- Zadania `adhoc` i `triggered`: zawsze „Wszyscy".
- Zadanie jest wspólne: odhacza pierwsza osoba, która je zrobi. Bez przypisań i rotacji w MVP.

Ekran ustawień pobiera dostępne usługi `notify.mobile_app_*` z HA i pozwala przypiąć je do osób. Przycisk „Wyślij testowe powiadomienie" przy każdej osobie.

## 5. Powiadomienia

Wysyłka: `POST http://supervisor/core/api/services/notify/<usługa>` z nagłówkiem `Authorization: Bearer $SUPERVISOR_TOKEN` i `Content-Type: application/json` **[docs]**. Body to bezpośrednio `service_data` usługi (obiekt JSON poniżej, bez dodatkowego opakowania); odpowiedź to lista zmienionych stanów **[docs]**.

Payload:
```json
{
  "title": "Obowiązki",
  "message": "Wyniesienie śmieci",
  "data": {
    "tag": "chores-occ-123",
    "actions": [
      {"action": "CHORES_DONE_123_p2", "title": "Zrobione"},
      {"action": "CHORES_SNOOZE_123_p2", "title": "Za godzinę"}
    ]
  }
}
```

Zasady:
- Android pozwala na maks. **3 akcje** w powiadomieniu **[docs]**.
- Identyfikator akcji zawiera id wystąpienia i id osoby (`_p2`), bo powiadomienia są wysyłane per osoba. Dzięki temu wiadomo, kto kliknął, bez polegania na `context.user_id`.
- Stały `tag` per wystąpienie: ponowienie zastępuje poprzednie powiadomienie zamiast tworzyć kolejne **[docs]**. Na Androidzie nie używać tego samego `tag` w różnych `group` **[docs]**.
- Kasowanie po odhaczeniu: `message: "clear_notification"` i `data.tag` z tym samym tagiem, do wszystkich odbiorców **[docs]**. Zastrzeżenie z docs: na Androidzie dla powiadomień, które nie są krytyczne, skasowanie może wymagać, żeby aplikacja była niedawno używana — kasowanie jest „best effort”, a stan prawdy trzyma baza.
- Tapnięcie w treść otwiera appkę: `data.clickAction` (Android; `url` dla iOS) przyjmuje względny URL instancji HA, np. `/lovelace/test` **[docs]**. Dokumentacja nie podaje ścieżki panelu ingress appki — w M0 odczytać ją z paska adresu po otwarciu panelu „Obowiązki” i zapisać w spec (slug zawiera prefiks repozytorium, patrz `/addons/self/info`, dostępne bez `hassio_api`) **[docs: względny URL; ścieżka panelu do ustalenia w M0]**.
- Dostarczanie na czas: `ttl: 0`, `priority: high` w `data` — bez tego Android potrafi nie zadzwonić, gdy telefon leży z wygaszonym ekranem (nie dotyczy lokalnego push) **[docs]**. Osobny `channel` „Obowiązki": kanał tworzy się przy pierwszym powiadomieniu z daną nazwą, potem użytkownik ustawia jego dźwięk w Androidzie (domyślnie `General`) **[docs]**. Właściwości kanału (`importance`, `vibrationPattern`, `ledColor`) są ustawiane tylko przy pierwszym użyciu kanału, później ignorowane (poza obniżeniem `importance`) — dlatego `importance: high` trzeba wysłać od pierwszego powiadomienia **[docs]**.
- Powiadomienia informacyjne („ktoś zrobił") idą bez przycisków, z osobnym tagiem, na osobnym kanale „Obowiązki – info".

Odbiór kliknięć: WebSocket `ws://supervisor/core/websocket`, uwierzytelnienie tokenem `SUPERVISOR_TOKEN` **[docs]**. Przebieg **[docs]**: serwer wysyła `{"type":"auth_required"}`, appka odpowiada `{"type":"auth","access_token":"<SUPERVISOR_TOKEN>"}`, serwer `auth_ok` (albo `auth_invalid` i rozłącza). Potem `{"id":N,"type":"subscribe_events","event_type":"mobile_app_notification_action"}` → `result` z `success: true`, a każde kliknięcie przychodzi jako `{"id":N,"type":"event","event":{"event_type":...,"data":{"action":...,"tag":...},"context":{"user_id":...}}}`. Kolejne komendy muszą mieć rosnące `id`. Zdarzenie niesie `action` oraz, na Androidzie, pola wysłane w powiadomieniu, m.in. `tag` **[docs]**. Wymagany auto-reconnect z backoffem; po reconnect appka odczytuje aktualne stany obserwowanych encji, żeby nie zgubić końca cyklu.

Akcje:
- `DONE` → wystąpienie `done`, zapis kto/kiedy, kasowanie przypomnień, powiadomienie informacyjne (jeśli włączone dla zadania).
- `SNOOZE` → tylko dla tej osoby, następne przypomnienie za 60 min. Pozostałe osoby bez zmian.
- `ME` (tryb `ask_who`) → zapis wykonania na klikającego, kasowanie pytania u pozostałych.

## 6. Scheduler

- Tick co minutę. Decyzja „co wysłać" wynika wyłącznie ze stanu w bazie (wystąpienie + `last_notified_at` per osoba), więc restart niczego nie gubi i niczego nie dubluje.
- Po przestoju: maks. jedno zaległe powiadomienie na wystąpienie, bez serii.
- Generowanie wystąpień: `dated` na dany dzień o północy (lub przy starcie), `weekly` w poniedziałek 00:00.
- Strefa czasowa z HA (`GET /api/config` → `time_zone`, np. `"Europe/Zurich"`; także komenda WS `get_config`) **[docs]**. Obraz musi mieć dane stref (w Go: `import _ "time/tzdata"`).
- Okno powiadomień globalne: `reminders_start` 10:00, `reminders_end` 20:00.

## 7. Model danych (SQLite w `/data/chores.db`)

`/data` to trwały wolumen appki **[docs]**.

- `person(id, name, color, ha_user_id, active)`
- `person_device(id, person_id, notify_service)`
- `recipient_list(id, name, is_builtin_all)` + `recipient_list_member(list_id, person_id)`
- `task(id, name, type[dated|weekly|adhoc|triggered], recurrence_json, recipient_list_id, notify_on_done, weight, counts_in_stats, catalog_key, active, created_at)`
- `appliance(id, name, detect_kind[state|power], entity_id, detect_config_json, fsm_state, fsm_changed_at)`
- `trigger(id, appliance_id, on_event[cycle_start|cycle_end], task_id, mode[ask_who|needs_doing])`
- `occurrence(id, task_id, due_date, week_start, status[open|done|missed|skipped], source[schedule|manual|trigger], requested_by, requested_at, done_by, done_at, weight_at_done)`
- `weight_change(id, task_id, old_weight, new_weight, changed_at, source[manual|import], reason)`
- `notification_state(occurrence_id, person_id, last_notified_at, snoozed_until)`
- `settings(key, value)`

Uwagi:
- `done_by` może być puste (wpisy „Nieprzypisane").
- `notify_on_done`: domyślnie `true` dla `adhoc` i `triggered`, `false` dla `dated` i `weekly`.
- `counts_in_stats`: domyślnie `true`; pozwala wyłączyć zadanie z porównań.

### 7.1 Wagi nie działają wstecz

- `task.weight` to waga **bieżąca**, liczba całkowita 1–10, domyślnie 1.
- W chwili odhaczenia appka kopiuje ją do `occurrence.weight_at_done`. Wszystkie porównania, kalendarz i eksport liczą wyłącznie z `weight_at_done`, nigdy z `task.weight`.
- Zmiana wagi (ręczna lub z importu) obowiązuje od momentu zapisu i dotyczy tylko wykonań odhaczonych później. Minione okresy nie zmieniają wyniku.
- Poprawka wpisu w historii (zmiana osoby) zachowuje jego `weight_at_done`. Wykonanie dopisane ręcznie z datą wsteczną dostaje wagę, która obowiązywała w tej dacie, odczytaną z `weight_change`.
- Każda zmiana trafia do `weight_change` razem ze źródłem i uzasadnieniem; historia zmian jest widoczna przy zadaniu.

## 8. UI

Mobile-first, po polsku, otwierane w aplikacji HA przez ingress. Wszystkie URL-e względne; ścieżka bazowa z nagłówka `X-Ingress-Path` **[docs]**. Ingress przenosi HTTP/1.x, streaming i WebSockety **[docs]**. Rozpoznanie zalogowanego użytkownika HA z nagłówków, które Supervisor dodaje do każdego żądania ingress: `X-Remote-User-Id`, `X-Remote-User-Name`, `X-Remote-User-Display-Name` **[docs]**. Fallback (brak nagłówka albo użytkownik niepowiązany z osobą): wybór osoby zapamiętany w `localStorage`.

Ekrany:
1. **Do zrobienia** — otwarte wystąpienia: dziś i zaległe, potem „w tym tygodniu". Duży przycisk „Zrobione" przy każdym. Sekcja „Nieprzypisane" z przyciskiem „To ja".
2. **Szybkie** — kafelki zadań `adhoc` z przyciskami „Trzeba zrobić" / „Zrobione" i informacją o ostatnim wykonaniu.
3. **Kalendarz** — widok miesiąca. W każdym dniu kropki w kolorach osób za wykonane zadania i znacznik pominiętych. Tapnięcie dnia pokazuje listę: co, kto, o której, co przepadło. Dni przyszłe pokazują zaplanowane zadania `dated`. Nawigacja między miesiącami.
4. **Porównanie** — przełącznik okresu: tydzień / 2 tygodnie / miesiąc, ze strzałkami do poprzednich okresów. Dla każdej osoby: liczba wykonań, suma ważona, udział procentowy, poziome paski w kolorze osoby. Pod spodem tabela zadanie × osoba. Osobno liczba zadań pominiętych i nieprzypisanych. Paski w czystym CSS, bez biblioteki wykresów. Przycisk „Eksport dla LLM" (pkt 8.1).
5. **Zadania** — lista i edycja definicji (nazwa, typ, powtarzalność, odbiorcy, waga, czy liczyć w porównaniu, powiadomienie o wykonaniu), katalog z pkt 3 oraz widok „Wagi": jedna tabela wszystkich zadań z polem wagi, do szybkiej ręcznej zmiany wielu naraz, z przyciskiem „Importuj wagi" (pkt 8.2).
6. **Ustawienia** — osoby i telefony, listy odbiorców, urządzenia i wyzwalacze, okno powiadomień, interwał ponowień, harmonogram narastania, test powiadomienia.

Wpis w historii da się poprawić: zmienić osobę albo usunąć błędne odhaczenie.

### 8.1 Eksport miesiąca dla LLM

Cel: raz na jakiś czas wkleić dane do dowolnego LLM, dostać propozycję nowych wag i wczytać ją z powrotem (pkt 8.2). Appka sama nie woła żadnego modelu; wklejanie w obie strony robi człowiek.

Przycisk na ekranie „Porównanie" → wybór miesiąca → appka generuje jeden tekst Markdown z trzech części:

1. **Instrukcja** — treść edytowalna w ustawieniach, domyślna wbudowana. Prosi o ocenę, czy wagi oddają realny wysiłek (czas, uciążliwość, częstotliwość), o nowe wagi w skali 1–10 i o wskazanie zadań, które wyglądają na brakujące lub zdublowane.
2. **Wymagany format odpowiedzi** — stały, doklejany przez appkę, nieedytowalny, żeby import zawsze działał.
3. **Dane** — blok JSON.

Dane w eksporcie:
- `period` (np. `2026-09`) i `exported_at`
- lista aktywnych zadań: `task_id`, nazwa, typ, aktualna waga, czy liczone w porównaniu
- dla każdego zadania: liczba wykonań per osoba, liczba pominiętych i nieprzypisanych
- podsumowanie per osoba: liczba wykonań, suma ważona (z `weight_at_done`), udział procentowy
- osoby pod nazwami z appki, bez identyfikatorów HA i nazw urządzeń

Wymagany format odpowiedzi, wpisany w prompt dosłownie razem z przykładem: dokładnie jeden blok kodu JSON, bez tekstu przed nim i po nim.

```json
{
  "schema": "obowiazki-weights/1",
  "period": "2026-09",
  "weights": [
    {
      "task_id": 12,
      "name": "Sprzątanie łazienki",
      "current_weight": 1,
      "proposed_weight": 6,
      "reason": "Zajmuje około 40 minut i jest najbardziej uciążliwe z tygodniowych."
    }
  ],
  "notes": ["Brakuje zadania: wymiana worka w odkurzaczu."]
}
```

Reguły formatu podane w prompcie:
- `weights` zawiera **każde** zadanie z danych, także te bez zmiany (wtedy `proposed_weight` = `current_weight`)
- `task_id`, `name` i `current_weight` przepisane z danych bez zmian
- `proposed_weight`: liczba całkowita 1–10
- `reason`: jedno zdanie po polsku
- `notes`: uwagi o brakujących lub zdublowanych zadaniach, wyłącznie informacyjne

Wydanie tekstu: przycisk „Kopiuj" oraz „Pobierz .md". Schowek w WebView aplikacji HA bywa zablokowany **[trening — dokumentacja HA tego nie opisuje; sprawdzić na telefonie w M5]**, więc tekst jest też widoczny w polu tekstowym do ręcznego zaznaczenia.

### 8.2 Import wag

Przycisk „Importuj wagi" w widoku „Wagi" → pole tekstowe do wklejenia odpowiedzi LLM albo wybór pliku.

Parsowanie, tolerancyjne na typowe odstępstwa modeli:
- appka wyciąga pierwszy obiekt JSON z tekstu, także gdy jest otoczony ogrodzeniem ```` ```json ```` albo dodatkowym komentarzem
- `schema` musi być `obowiazki-weights/1`; inna wartość lub niepoprawny JSON = czytelny błąd, nic nie jest zmieniane

Walidacja wiersz po wierszu:
- `task_id` nie istnieje lub zadanie jest nieaktywne → wiersz pominięty, z ostrzeżeniem
- `name` różni się od nazwy w bazie → ostrzeżenie (dopasowanie idzie po `task_id`, nie po nazwie)
- `proposed_weight` nie jest liczbą całkowitą 1–10 → wiersz odrzucony
- `current_weight` różni się od wagi w bazie → ostrzeżenie „waga zmieniła się od eksportu"
- zadania z bazy nieobecne w odpowiedzi → zostają bez zmian

Potem **podgląd**, bez którego nic się nie zapisuje: tabela `zadanie | obecnie | propozycja | uzasadnienie`, tylko wiersze ze zmianą, każdy z polem wyboru (domyślnie zaznaczone) i możliwością ręcznej korekty propozycji. Pod tabelą `notes` jako lista do przeczytania. Przycisk „Zastosuj zaznaczone" zapisuje wagi w jednej transakcji i dodaje wpisy `weight_change` ze źródłem `import` i uzasadnieniem z `reason`.

Zgodnie z pkt 7.1 zaimportowane wagi obowiązują od chwili zastosowania, a miesiąc, który był analizowany, zachowuje stare wyniki.

## 9. Technikalia appki

Uwaga nazewnicza: add-ony w HA nazywają się teraz **apps**, dokumentacja jest pod `/docs/apps` **[docs]**.

Stack (lekki): **Go**, jeden binarny plik; SQLite przez `modernc.org/sqlite` (bez CGO); frontend statyczny osadzony przez `embed` (czysty HTML + htmx lub Alpine.js, bez kroku budowania Node). Docelowo obraz ~20 MB, RAM kilkanaście MB.

`config.yaml` (klucze potwierdzone **[docs]**):
```yaml
name: "Obowiązki"
version: "0.1.0"
slug: obowiazki
description: "Domowe obowiązki z przypomnieniami przez aplikację Home Assistant"
arch:
  - amd64
  - aarch64
ingress: true
ingress_port: 8099
panel_icon: mdi:broom
panel_title: "Obowiązki"
panel_admin: false        # domyślnie true = tylko admini widzą panel
homeassistant_api: true   # dostęp do http://supervisor/core/api
backup: cold              # zatrzymuje appkę na czas backupu, bezpieczne dla SQLite
```

Dockerfile **[docs]**:
- Od Supervisora 2026.04.0 argument `BUILD_FROM` nie jest już podawany domyślnie, a `build.yaml` nie jest używany. Obraz bazowy wpisz jawnie w `FROM`.
- Multi-stage: `golang:alpine` (build) → `alpine` (runtime) z samą binarką. Runtime nie potrzebuje `ca-certificates` (ruch tylko do `http://supervisor`) ani `tzdata` (wbudowane w binarkę).
- `modernc.org/sqlite` w aktualnej wersji wymaga Go 1.26, stąd `golang:1.26-alpine`. Obraz M0 po rozpakowaniu ma ok. 33 MB (alpine ok. 13 MB + binarka).
- Obsługiwane architektury to tylko `aarch64` i `amd64`.

Bezpieczeństwo ingress: serwer nasłuchuje na 8099 (domyślny port ingress; inny wymaga `ingress_port`) i przyjmuje połączenia tylko z adresu bramy ingress `172.30.32.2`, odrzucając wszystkie inne **[docs]**.

### 9.1 Repozytorium i instalacja

Repo `Fryk/house-chores-ha` jest jednocześnie repozytorium appek HA. Układ:

```text
repository.yaml        # nazwa i opis repozytorium appek
obowiazki/             # katalog appki (nazwa = slug)
  config.yaml
  Dockerfile
  DOCS.md, README.md, CHANGELOG.md, icon.png
  cmd/, internal/, web/  # kod Go i osadzony frontend
SPEC.md
```

`repository.yaml` w katalogu głównym jest wymagany; klucz `name` obowiązkowy, `url` i `maintainer` opcjonalne **[docs]**.

Instalacja: Ustawienia → Apps → „App store” (prawy dolny róg) → dodanie URL repozytorium w polu repozytoriów → Zapisz **[docs]**. Dokładne etykiety menu w aktualnym UI potwierdzić przy instalacji w M0. Uwaga: dokumentacja nie opisuje instalacji z prywatnego repo **[trening — przyjmujemy, że Supervisor go nie sklonuje bez dodatkowej konfiguracji]**. Jeśli repo ma zostać prywatne, alternatywą jest lokalna appka: skopiowanie katalogu `obowiazki/` do podkatalogu `/local_apps` na hoście HA (przez appkę Samba — udział `local_apps` — albo SSH), potem w sklepie ⋮ → „Check for updates” i appka pojawia się w sekcji „Local apps” **[docs]**. Dawna nazwa katalogu `/addons` nie jest już używana w docs.

### 9.2 Tryby uruchamiania

Tylko dwa:

| Tryb | Kiedy | Do czego |
|---|---|---|
| **supervisor** | automatycznie, gdy jest zmienna `SUPERVISOR_TOKEN` | Docelowy, jako appka w HA. |
| **fake** | gdy `SUPERVISOR_TOKEN` nie ma | Lokalnie, bez HA: `go run` albo `docker run`. Powiadomienia lądują w logu i na stronie `/dev`, gdzie można kliknąć ich przyciski oraz zasymulować start i koniec cyklu urządzenia. Zegar da się przesunąć, żeby testować harmonogramy. Strona `/dev` nie istnieje w trybie supervisor. |

W trybie fake: `DATA_DIR` zamiast `/data`, `PORT` zamiast 8099, ścieżka bazowa `/`, osoba wybierana ręcznie (fallback z pkt 8), przykładowe dane ładowane flagą `--seed`. Klient HA siedzi za interfejsem z dwiema implementacjami; logika schedulera, powtarzalności i automatu urządzeń nie zależy od HA.

## 10. Założenia do potwierdzenia przez właściciela

Przyjęte domyślnie, łatwe do zmiany:
1. Ponawianie przypomnień co 3 h w oknie 10:00–20:00.
2. Zgłoszenie lub koniec cyklu urządzenia poza oknem czeka z powiadomieniem do 10:00. Typowy przypadek: zmywarka kończy w nocy, przypomnienie o rozładowaniu przychodzi o 10:00. Alternatywa: osobny, wcześniejszy start okna dla zadań z urządzeń.
3. Zadania tygodniowe przypominane jednym zbiorczym powiadomieniem, harmonogram z pkt 2.2.
4. Tydzień pon.–niedz.; niewykonane zadanie tygodniowe przepada (`missed`), nie przechodzi na kolejny tydzień.
5. Powiadomienie „ktoś zrobił" domyślnie dla zadań bez terminu i z urządzeń; dla pozostałych do włączenia per zadanie.
6. Powiadomienie „ktoś zrobił" trafia do wszystkich poza osobą, która wykonała (przełącznik w ustawieniach).
7. „Za godzinę" odkłada przypomnienie tylko klikającemu.
8. Pralka: pytamy kto włączył i kto wyjął. Suszarka i zmywarka: tylko kto wyjął.
9. Pytanie „Kto włączył?" jest wysyłane raz, bez ponawiania.
10. Porównanie liczy wykonania oraz sumę ważoną; wszystkie wagi startują od 1, skala 1–10.
11. Nowa waga obowiązuje od chwili zapisu, bez możliwości ustawienia daty „od kiedy". Wagi z importu po analizie września zaczynają działać w dniu importu, a nie od 1 października.

## 11. Kamienie milowe

- **M0 — szkielet.** Układ repo z pkt 9.1, baza SQLite z migracjami (na razie `settings` i tymczasowy dziennik zdarzeń), appka działa w trybie fake ze stroną `/dev`, instaluje się w HA, panel „Obowiązki" otwiera się przez ingress w aplikacji HA na telefonie, przycisk wysyła testowe powiadomienie z akcją, a kliknięcie akcji jest widoczne w logu appki. Ten etap weryfikuje wszystkie punkty **[trening]** — po nim zaktualizuj spec.
- **M1 — osoby, katalog i zadania bez terminu.** Ustawienia osób/telefonów/list, katalog startowy, ekran „Szybkie", oba przepływy z pkt 2.3 łącznie z kasowaniem powiadomień i powiadomieniem informacyjnym.
- **M2 — zadania na dzień.** Powtarzalność, generowanie wystąpień, scheduler, ponawianie, snooze, zaległe.
- **M3 — zadania tygodniowe.** Zbiorcze powiadomienie i harmonogram narastania.
- **M4 — urządzenia.** Konfiguracja urządzeń, automat cyklu (stan i moc), tryby `ask_who` i `needs_doing`, „Nieprzypisane".
- **M5 — kalendarz i porównanie.** Oba ekrany, edycja wpisów historii, widok „Wagi", eksport dla LLM, import wag z podglądem.
- **M6 — dopracowanie.** Odporność na restart i utratę WebSocketu, test backupu i przywrócenia.

Testy jednostkowe z wstrzykiwanym zegarem: scheduler i powtarzalność (zmiana czasu letni/zimowy w Europe/Warsaw, restart w środku okna, przestój przez noc, granica 20:00), automat urządzeń (skoki stanu, cykl kończący się w nocy, nowy cykl przy otwartym „wyjęciu"), agregacje porównania (granice tygodnia i miesiąca; zmiana wagi w środku okresu nie rusza wcześniejszych wykonań), import wag (JSON w ogrodzeniu i z komentarzem wokół, zły `schema`, nieznany `task_id`, waga spoza 1–10, rozjechane `current_weight`).

## 12. Źródła

- App configuration: https://developers.home-assistant.io/docs/apps/configuration
- App communication: https://developers.home-assistant.io/docs/apps/communication
- Actionable notifications: https://companion.home-assistant.io/docs/notifications/actionable-notifications
- Notifications basic (tag, clear_notification, clickAction, channel): https://companion.home-assistant.io/docs/notifications/notifications-basic
- Critical notifications (ttl/priority na Androidzie): https://companion.home-assistant.io/docs/notifications/critical-notifications
- REST API: https://developers.home-assistant.io/docs/api/rest
- WebSocket API: https://developers.home-assistant.io/docs/api/websocket
- App presentation (ingress, `X-Ingress-Path`, 172.30.32.2): https://developers.home-assistant.io/docs/apps/presentation
- App security (nagłówki `X-Remote-User-*`): https://developers.home-assistant.io/docs/apps/security
- App repository (`repository.yaml`, instalacja): https://developers.home-assistant.io/docs/apps/repository
- Lokalne appki (`/local_apps`): https://developers.home-assistant.io/docs/apps/tutorial, https://developers.home-assistant.io/docs/apps/testing

Weryfikacja 2026-10-11 była robiona na źródłach Markdown tych stron (repozytoria `home-assistant/developers.home-assistant` i `home-assistant/companion.home-assistant`, gałąź `master`), bo strony docs były niedostępne z sieci środowiska.
