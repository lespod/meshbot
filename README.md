> Ta wersja Meshbota jest przepisaniem aplikacji na Go. Jeśli z jakiegoś powodu
> naprawdę potrzebujesz starej, popsutej wersji w Pythonie, została zachowana w
> gałęzi
> [legacy-version-in-python](https://github.com/Timendus/meshbot/tree/legacy-version-in-python).

# Meshbot

Prosty bot do użycia z Meshtastic. Nazwa nie jest szczególnie oryginalna.

Część osób nazwałaby to „BBS-em”, ale bliżej mu do prostego bota obsługującego
komendy przez sieć Meshtastic.

Aplikacja jest napisana w Go, dzięki czemu jest lekka w porównaniu z podobnymi
programami pisanymi w Pythonie. Binarka i obraz Dockera mają po kilka
megabajtów. Do działania potrzebuje niewiele CPU i tylko kilku megabajtów RAM.
Powinna też działać stabilnie.

## Aktualne funkcje

- Raporty sygnału i lista sąsiadów.
- Diagnostyka przeskoków, odległości i trasy między nodami.
- Raporty pogodowe i prognozy z
  [open-meteo.com](https://open-meteo.com/) (bot potrzebuje połączenia z
  Internetem).
- Programowalne, cykliczne ogłoszenia na kanałach, np. dla komunikatów
  lokalnej społeczności.

## Użycie w meshu

Instrukcje używania Meshbota przez Meshtastic są w [manualu](./manual.md).

## Hosting Meshbota

### Odpowiedzialność

Meshtastic ma bardzo małą przepustowość. Jeśli używasz tego bota, a szczególnie
jeśli chcesz go modyfikować, dopilnuj, żeby nie spamował lokalnego mesha. Bot
powinien mówić tylko wtedy, gdy ktoś się do niego odezwie.

Krótko: bądź dobrym sąsiadem w meshu.

### Konfiguracja

> **Uwaga**: bot był dotąd testowany głównie na Linuksie i jako obraz Dockera,
> przez TCP. Prawdopodobnie zadziała też przez USB oraz na macOS, Windowsie albo
> Raspberry Pi, ale szerokie wsparcie tych wariantów nie jest obecnie
> priorytetem.

Potrzebujesz noda Meshtastic i komputera, na którym będzie działał bot. Node i
komputer mogą być połączone kablem USB albo przez sieć, np. przez
[Wi-Fi lub Ethernet](https://meshtastic.org/docs/configuration/radio/network/).

USB może być bardziej mobilne i nie zależy od lokalnej sieci, np. podczas awarii
zasilania. Połączenie sieciowe pozwala postawić noda w najlepszym miejscu dla
odbioru, a bota uruchomić tam, gdzie masz dostępny komputer.

#### Node Meshtastic

Ten projekt jest rozwijany na Heltec v3, ale dowolny node Meshtastic powinien
być wystarczający.

Upewnij się, że poza botem żaden inny klient nie komunikuje się z tym nodem. W
przeciwnym razie oba klienty mogą gubić wiadomości i całość będzie wyglądała na
zepsutą. Odłącz aplikację mobilną i nie zestawiaj innych połączeń z nodem, gdy
bot działa.

Praktyczne wskazówki po stronie Meshtastic:

- Dodaj emoji robota (🤖) do nazwy noda, żeby inni widzieli, że to bot.
- W aplikacji Meshtastic na Androidzie możesz dodać quick messages z komendami,
  np. `/test` i `/sygnal`, żeby wysyłać je jednym kliknięciem.

#### Komputer

Wystarczy dowolny komputer, o ile pozostaje włączony. Bot może działać na NAS-ie
albo nawet na starszym Raspberry Pi. Możesz uruchomić go bezpośrednio albo przez
Dockera.

Pobierz właściwą wersję z
[releases](https://github.com/Timendus/meshbot/releases). Edytuj `config.json`,
żeby wskazać botowi, jak połączyć się z nodem i jak ma się zachowywać.
`config.json` powinien znajdować się w tym samym katalogu co program.

Dla Dockera zamontuj katalog do `/app/config` i uruchom kontener. Przy pierwszym
starcie, jeśli wszystko jest poprawnie skonfigurowane, w zamontowanym katalogu
powstanie `config.json`. Zatrzymaj kontener, edytuj konfigurację i uruchom go
ponownie.

## Lokalny development

Zależności:

- Go
- Git
- make

Przykład:

```bash
git clone git@github.com:Timendus/meshbot.git
cd meshbot
vi config.json
make
```

Po tym bot powinien się uruchomić.
