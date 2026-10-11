# Obowiązki

[English](#english) · [Polski](#polski)

## English

### What works now (M0)

- The **Obowiązki** panel opens in the Home Assistant sidebar (also for non-admin users).
- **Test notification:** pick your phone's `notify.mobile_app_*` service and press *Send test notification*. The notification has two buttons, *Done* and *In 1 hour*.
- Tapping a button shows up in the app log (*Log* tab) and in *Recent events*. *Done* also removes the notification from the phone.
- *Ingress diagnostics* shows what Home Assistant passes to the app (base path, signed-in user).

The UI language follows your browser or phone language; use the switch in the header to change it.

### Path opened by tapping a notification

The Home Assistant documentation does not say which path opens an app panel. Open the **Obowiązki** panel, copy the path from the address bar (the part after the host, e.g. `/…_obowiazki`) and paste it into the *clickAction* field. The value is saved.

### Data

Data is kept in SQLite in the app's `/data` folder. Backups stop the app for a moment (`backup: cold`) so the database stays consistent.

### Running locally (fake mode)

Without the `SUPERVISOR_TOKEN` variable the app runs in fake mode: notifications go to the log and to the `/dev` page, where you can tap their buttons.

```sh
DATA_DIR=./data PORT=8080 go run ./cmd/obowiazki
```

## Polski

### Co działa teraz (M0)

- Panel **Obowiązki** otwiera się w menu bocznym Home Assistanta (także dla użytkowników bez uprawnień administratora).
- **Testowe powiadomienie:** wybierz usługę `notify.mobile_app_*` swojego telefonu i kliknij *Wyślij testowe powiadomienie*. Powiadomienie ma dwa przyciski: *Zrobione* i *Za godzinę*.
- Kliknięcie przycisku widać w logu appki (zakładka *Log*) i w *Ostatnich zdarzeniach*. *Zrobione* dodatkowo usuwa powiadomienie z telefonu.
- *Diagnostyka ingress* pokazuje, co Home Assistant przekazuje appce (ścieżka bazowa, zalogowany użytkownik).

Język interfejsu wynika z ustawień przeglądarki lub telefonu; można go zmienić przełącznikiem w nagłówku.

### Ścieżka otwierana po tapnięciu w powiadomienie

Dokumentacja Home Assistanta nie podaje, jaka ścieżka otwiera panel appki. Otwórz panel **Obowiązki**, skopiuj ścieżkę z paska adresu (część po nazwie hosta, np. `/…_obowiazki`) i wklej w pole *clickAction*. Wartość zostanie zapamiętana.

### Dane

Dane są w bazie SQLite w katalogu `/data` appki. Na czas backupu appka jest na chwilę zatrzymywana (`backup: cold`), żeby baza była spójna.

### Uruchomienie lokalne (tryb fake)

Bez zmiennej `SUPERVISOR_TOKEN` appka działa w trybie fake: powiadomienia trafiają do logu i na stronę `/dev`, gdzie można klikać ich przyciski.

```sh
DATA_DIR=./data PORT=8080 go run ./cmd/obowiazki
```
