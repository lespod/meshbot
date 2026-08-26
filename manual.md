# Jak używać Meshbota przez mesh

Jako użytkownik Meshbota możesz wysłać przez Meshtastic dowolną z poniższych
komend, a bot odpowie.

Komendy nie rozróżniają wielkości liter.

Administrator może włączać i wyłączać reakcje bota na poszczególne komendy w
sekcji `commands` pliku `config.json`. Brak sekcji `commands` albo brak
konkretnego wpisu oznacza, że dana komenda jest włączona.

## Komendy

### Na kanale albo jako wiadomość prywatna

- `/pomoc` albo `/o` - krótki opis komend
- `/ping` - test połączenia z botem
- `/sygnal <opcjonalny node>` - raport sygnału dla Ciebie albo dla wskazanego
  noda
- `/sasiedzi` - lista nodów, które bot słyszy przez LoRa
- `/hopy` - liczba przeskoków między Tobą a botem oraz miejscowość, z której
  bot odpowiada na podstawie własnej lokalizacji
- `/test` albo `test` - diagnostyka: liczba przeskoków, miejscowość bota i
  odległość między nodami, jeśli pozycje obu nodów są znane. Odpowiedź jest
  oznaczona jako odpowiedź na konkretną wiadomość, jeśli klient Meshtastic to
  obsługuje
- `/pogoda` - aktualne warunki pogodowe
- `/prognoza` - prognoza pogody na najbliższe dni

Odpowiedzi są wysyłane jak zwykłe wiadomości Meshtastic: na kanał, na którym
wysłano komendę, albo jako DM z powrotem do Ciebie.

Alias z polskimi znakami (`/sygnał`, `/sąsiedzi`) oraz angielskie aliasy
(`/help`, `/about`, `/signal`, `/neighbours`, `/hops`, `/weather`, `/forecast`)
nadal działają.

### Tylko jako wiadomość prywatna

- `/sciezka` albo `/ścieżka` - traceroute z bota do Twojego noda. Wynik jest
  krótki i pokazuje trasę tam oraz z powrotem, jeśli firmware ją zwróci

Aliasami są również `/trasa`, `/trace` i `/traceroute`.

## Włączanie i wyłączanie komend

Przykład:

```json
"commands": {
  "ping": true,
  "pomoc": true,
  "sygnal": true,
  "sasiedzi": true,
  "hopy": true,
  "test": false,
  "sciezka": true,
  "pogoda": true,
  "prognoza": true
}
```

Wyłączenie komendy wyłącza też jej aliasy. Przykładowo `"test": false` wyłącza
zarówno `/test`, jak i `test`; `"sygnal": false` wyłącza `/sygnal`, `/sygnał`
i `/signal`.

Komendy inne niż wymienione powyżej są ignorowane. Zwykłe wiadomości DM nie są
zapisywane ani przekazywane dalej przez Meshbota.

## Logowanie

Rodzaje logów można włączać i wyłączać w sekcji `logging` pliku `config.json`:

```json
"logging": {
  "incoming_messages": false,
  "outgoing_messages": false,
  "connections": true,
  "channels": false,
  "announcements": true,
  "protocol_packets": false,
  "acknowledgements": false
}
```

Domyślnie wyłączone są pełne wiadomości, surowe pakiety protokołu i szczegóły
potwierdzeń. Błędy są zawsze zapisywane w logu.

## Kanały i wiadomości prywatne

Wiadomości prywatne mają informację zwrotną o doręczeniu w aplikacji, więc
widzisz, czy wiadomość dotarła do odbiorcy. Kanały pokazują tylko, że wiadomość
została przez kogoś powtórzona. Od Meshtastic 2.6 wiadomości prywatne korzystają
też z routingu „next-hop”; kanały nie mają z tego korzyści.

Komendy mogą odpowiadać na kanale albo prywatnie, zależnie od miejsca, w którym
zostały wysłane. `/sciezka` działa wyłącznie prywatnie, ponieważ ujawnia trasę
przez sieć mesh.
