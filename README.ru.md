# lm-bridge

[English](README.md) | [Русский](README.ru.md)

Приложение для macOS (menubar) + CLI, которое подключает [Claude Code](https://claude.ai/claude-code) к языковой модели — локальной через [LM Studio](https://lmstudio.ai) или облачной через [OpenRouter](https://openrouter.ai).

Делегируй механические задачи (поиск по коду, генерация шаблонов, трансформации) вспомогательной модели — оставляя контекст Claude свободным для настоящего мышления.

![lm-bridge dashboard](docs/screenshot.png)

## Возможности

- **Menubar-приложение** — живой дашборд с историей вызовов, токенами, латентностью и прогрессом активной задачи
- **CLI** — команды `query`, `agent`, `review`, `explain`, `status`
- **Настройки провайдера** — переключение между LM Studio (локально) и OpenRouter (облако) прямо в дашборде
- **Интеграция с Claude Code** — автоматически добавляет инструкции по использованию в твой `CLAUDE.md`
- **Режим агента** — модель сама читает файлы через tool calls, без ручного копирования
- **Code review** — ревью git diff перед коммитом
- **Explain** — структурированное объяснение любого файла
- **Стриминг** — флаг `--stream` с детектором зависаний (автоматически останавливает зациклившуюся генерацию)
- **Отслеживание активной задачи** — прогресс-бар и кнопка отмены прямо в дашборде
- **История вызовов** — провайдер и модель видны для каждого вызова

## Требования

- macOS (Apple Silicon рекомендуется)
- Один из вариантов:
  - [LM Studio](https://lmstudio.ai) запущен локально с загруженной моделью
  - API-ключ [OpenRouter](https://openrouter.ai) (доступны бесплатные модели)

## Установка

### Скачать

Скачай последний `.app` из [Releases](https://github.com/d-u-d/lm-bridge/releases).

### Собрать из исходников

```bash
# Требования: Go 1.22+, Wails v2, Node.js 18+
go install github.com/wailsapp/wails/v2/cmd/wails@latest

git clone https://github.com/d-u-d/lm-bridge
cd lm-bridge
./build.sh
```

## Настройка

### 1. Настройка провайдера

Открой `lm-bridge.app` и нажми **⚙ Settings**:

- **LM Studio** — укажи URL (по умолчанию: `http://localhost:1234/v1`)
- **OpenRouter** — вставь API-ключ, нажми "Load free models", выбери модель

### 2. Проверить соединение

```bash
lm-bridge status
# Provider:  openrouter
# Model:     google/gemma-3-12b-it:free
# Status:    ✓ ready
```

### 3. Включить интеграцию с Claude Code (опционально)

В дашборде нажми **Enable** рядом с "Claude Code Integration". lm-bridge добавит блок инструкций в `~/.claude/CLAUDE.md` — Claude будет знать когда и как делегировать задачи.

**Фраза-триггер:** скажи Claude **"привлеки помощника"** — он проверит статус и делегирует задачу автоматически.

## Использование

### CLI

```bash
# Проверить провайдер и статус соединения
lm-bridge status

# Простой запрос (поддерживается stdin)
lm-bridge query "объясни это" < file.txt

# Режим агента — модель сама читает файлы через tool calls
lm-bridge agent --dir /path/to/project "найди все TODO комментарии"

# Ревью git diff перед коммитом
lm-bridge review
lm-bridge review --staged   # только staged изменения

# Объяснение файла
lm-bridge explain internal/cli/agent.go
cat main.go | lm-bridge explain

# Стриминг с детектором зависаний
lm-bridge query --stream "напиши подробное объяснение..."

# Включить reasoning для сложных задач
lm-bridge agent --think --dir . "проанализируй этот модуль"
```

### Примеры

```bash
# Найти все места использования переменной
lm-bridge agent --dir . "найди все использования переменной окружения DATABASE_URL"

# Сгенерировать шаблонный код
lm-bridge agent --dir . "создай CRUD эндпоинты для модели User по существующим паттернам"

# Трансформация контента
cat api.go | lm-bridge query "добавь godoc комментарии ко всем экспортируемым функциям, верни только изменённый файл"

# Быстрое ревью перед коммитом
lm-bridge review --staged
```

## Как это работает

```
Claude Code  →  lm-bridge CLI  →  LM Studio (локально)
                      ↕               или
               SQLite (общее)     OpenRouter (облако)
                      ↕
               lm-bridge.app (дашборд)
```

- CLI и GUI используют общую SQLite базу данных для истории вызовов, настроек и состояния активной задачи
- Режим агента использует OpenAI-совместимые tool calls для чтения файлов
- Дашборд показывает провайдер, модель и латентность для каждого вызова

## Сборка релиза

```bash
./build.sh v0.6.0
# Бинарник: build/bin/lm-bridge.app
```

## Лицензия

MIT
