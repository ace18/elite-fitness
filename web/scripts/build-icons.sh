#!/usr/bin/env bash
# Rigenera le icone PWA da static/favicon.svg — unica sorgente.
# Serve solo quando cambia il logo: gli output sono committati.
#
#   ./scripts/build-icons.sh      (richiede ImageMagick)
#
# Due varianti, e non sono intercambiabili:
#  - "any"      → angoli arrotondati come nel favicon, usata così com'è
#  - "maskable" → sfondo a tutto campo, senza arrotondamenti: la maschera la
#                 applica il sistema (cerchio, squircle, …) e se gli angoli
#                 fossero già tondi verrebbero ritagliati due volte, lasciando
#                 aloni bianchi. Stessa cosa per l'icona iOS, che per giunta
#                 rende nera qualsiasi trasparenza.
set -euo pipefail
cd "$(dirname "$0")/.."

SRC=static/favicon.svg
TEAL='#0FB5AE'

render() { # <svg-file> <px> <out>
  magick -background none "$1" -depth 8 -strip "PNG32:$3"
  test -s "$3"
}

tmp=$(mktemp -d); trap 'rm -rf "$tmp"' EXIT

# La dimensione intrinseca del sorgente è 32px: senza width/height espliciti
# ImageMagick rasterizza a 32 e poi ingrandisce, e l'icona esce sfocata.
sized() { sed "s|<svg |<svg width=\"$1\" height=\"$1\" |" "$SRC" > "$tmp/any-$1.svg"; echo "$tmp/any-$1.svg"; }
# Il full-bleed si ricava dal favicon togliendo il raggio degli angoli, così
# non esiste un secondo file da tenere allineato a mano.
fullbleed() { sed -e "s|<svg |<svg width=\"$1\" height=\"$1\" |" -e 's| rx="9"||' "$SRC" > "$tmp/mask-$1.svg"; echo "$tmp/mask-$1.svg"; }

render "$(sized 192)" 192 static/icon-192.png
render "$(sized 512)" 512 static/icon-512.png
render "$(fullbleed 512)" 512 static/icon-maskable-512.png
render "$(fullbleed 180)" 180 static/apple-touch-icon.png

echo "icone rigenerate:"
ls -l static/icon-*.png static/apple-touch-icon.png | awk '{print "  " $NF, $5"b"}'
