#!/bin/sh
# Сборка с собственным окном программы — без браузера вообще.
#
# Что нужно один раз поставить:
#   sudo apt install libwebkit2gtk-4.1-dev
#
# Почему отдельный скрипт: эта сборка использует CGO и заголовки WebKit,
# поэтому она не кросс-компилируется под Windows. Обычная сборка
# (go build -o niimbot .) остаётся без CGO и собирается где угодно.
#
# Посредник в third_party/pkgconfig нужен потому, что библиотека окна
# запрашивает webkit2gtk-4.0, которого в Ubuntu 24.04 уже нет.

set -e
cd "$(dirname "$0")"

if ! pkg-config --exists webkit2gtk-4.1; then
    echo "Не найдены заголовки WebKit. Поставьте:" >&2
    echo "  sudo apt install libwebkit2gtk-4.1-dev" >&2
    exit 1
fi

PKG_CONFIG_PATH="$PWD/third_party/pkgconfig${PKG_CONFIG_PATH:+:$PKG_CONFIG_PATH}" \
    go build -tags webview -o niimbot .

echo "Готово: собран с собственным окном."
echo "Запуск: ./niimbot gui"
