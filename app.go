package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"lm-bridge/internal/db"
	"lm-bridge/internal/llm"
	"lm-bridge/internal/tray"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const integrationStartMarker = "<!-- lm-bridge:start -->"
const integrationEndMarker = "<!-- lm-bridge:end -->"

// integrationBlock is the CLAUDE.md section injected when integration is enabled.
var integrationBlock = integrationStartMarker + `
## Local LLM Helper (lm-bridge)

Локальная модель Qwen3.6-35B-A3B доступна через ` + "`lm-bridge`" + `.
**Всегда спрашивай пользователя, запущен ли LM Studio**, прежде чем использовать.

### Принцип делегирования

Локальная модель — это **сборщик данных и исполнитель механической работы**.
Ты получаешь обратно структурированный результат и сам решаешь что с ним делать.
Делегируй задачи где результат: детерминированный, легко проверяемый, или обратимый.

### Делегируй — поиск и сбор информации

- "Найди все файлы где импортируется ` + "`AuthService`" + `"
- "Собери все места где используется переменная окружения ` + "`DATABASE_URL`" + `"
- "Найди все TODO / FIXME / HACK комментарии в проекте"
- "Покажи все HTTP эндпоинты — методы и пути"
- "Найди все захардкоженные строки и числа"
- "Собери все зависимости из package.json / requirements.txt"
- "Найди все файлы больше 300 строк"

### Делегируй — генерация шаблонного кода

- "Создай CRUD эндпоинты для модели ` + "`User`" + ` по этой схеме"
- "Сгенерируй тесты-заглушки для всех функций в ` + "`auth.ts`" + `"
- "Напиши Dockerfile для Node.js 20 приложения"
- "Создай ` + "`.env.example`" + ` на основе всех ` + "`process.env`" + ` в проекте"
- "Сгенерируй SQL миграцию для добавления колонки ` + "`deleted_at`" + `"

### Делегируй — деплой и CI задачи

- "Запусти тесты и верни список упавших с сообщениями об ошибках"
- "Прогони ESLint / pylint и собери все нарушения с файлами и строками"
- "Проверь что все переменные из ` + "`.env.example`" + ` заданы в окружении"
- "Собери changelog из ` + "`git log`" + ` за последние 2 недели в формате markdown"

### Делегируй — трансформации

- "Переведи все комментарии в ` + "`utils/`" + ` с русского на английский"
- "Сконвертируй этот OpenAPI JSON в TypeScript интерфейсы"
- "Добавь JSDoc комментарии ко всем экспортируемым функциям в ` + "`api.ts`" + `"

### НЕ делегируй

- Поиск багов в нетривиальной логике
- Архитектурные решения и рефакторинг с пониманием всего проекта
- Всё что связано с безопасностью, аутентификацией, криптографией
- Любая задача где ошибка неочевидна и сложно проверить результат

### Правила работы с результатом

- **Просить только нужный фрагмент** — не полный объект/файл, а только то поле которое нужно заполнить.
- **Инжектить через string replace в сыром тексте** — никогда через ` + "`json.load`" + ` + ` + "`json.dump`" + `. Любая сериализация меняет форматирование оригинала.
- **Большой контекст (~180KB+) ненадёжен** — дробить на отдельные запросы по одному объекту.
- **Передавать файл через pipe** — ` + "`cat file.txt | lm-bridge query \"задача\"`" + `, не через agent если файл один.

### Ограничения — конкуренция за модель

- **Одна задача за раз** — LM Studio обслуживает один запрос. Если уже идёт генерация — НЕ запускать lm-bridge.
- **Ошибка "LM Studio is busy"** — дождаться завершения текущей задачи. Не ретраиться.
- **Фоновые задачи** — не назначать фоновые задачи с lm-bridge пока идёт длинная генерация.

### Мониторинг выполнения

lm-bridge выводит прогресс в stderr:
` + "```" + `
[lm] Prompt processing progress: 60.6%   ← промпт обрабатывается, модель жива
[lm] Prompt processing progress: 100.0%  ← начинается генерация (молчание — норма)
` + "```" + `
После 100% тишина — это нормально, идёт генерация токенов. Ждать.

### Как вызывать

` + "```" + `bash
# Для задач с файлами (модель сама читает через tool calls):
lm-bridge agent --dir /path/to/project "задача"

# Для простых запросов без файлов:
lm-bridge query "запрос"

# Передать содержимое файла напрямую:
cat file.txt | lm-bridge query "summarize this"

# Ревью git diff перед коммитом:
lm-bridge review
lm-bridge review --staged

# Объяснение файла:
lm-bridge explain path/to/file.go

# Включить reasoning для сложных задач:
lm-bridge agent --think --dir . "задача"

# Стриминг — токены идут в stdout сразу, детектор петель включён:
lm-bridge query --stream "запрос"
` + "```" + `

Результат возвращается в stdout — используй его как контекст для своих следующих действий.
` + integrationEndMarker

func claudeMdPath() string {
	return filepath.Join(os.Getenv("HOME"), ".claude", "CLAUDE.md")
}

func addClaudeIntegration() error {
	path := claudeMdPath()
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	content := string(data)
	if strings.Contains(content, integrationStartMarker) {
		return nil
	}
	content = strings.TrimRight(content, "\n") + "\n\n" + integrationBlock + "\n"
	return os.WriteFile(path, []byte(content), 0644)
}

func removeClaudeIntegration() error {
	path := claudeMdPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	content := string(data)
	startIdx := strings.Index(content, integrationStartMarker)
	if startIdx == -1 {
		return nil
	}
	endIdx := strings.Index(content, integrationEndMarker)
	if endIdx == -1 {
		return nil
	}
	before := strings.TrimRight(content[:startIdx], "\n")
	after := strings.TrimLeft(content[endIdx+len(integrationEndMarker):], "\n")
	result := before
	if after != "" {
		result += "\n\n" + after
	}
	return os.WriteFile(path, []byte(result+"\n"), 0644)
}

type App struct {
	ctx    context.Context
	store  *db.Store
	client *llm.Client
}

func NewApp(store *db.Store) *App {
	return &App{
		store:  store,
		client: llm.New(""),
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	enabled := a.GetIntegrationEnabled()
	icon := tray.MakeIcon()
	tray.Init(icon, enabled,
		func() { // "Open Dashboard"
			runtime.WindowShow(a.ctx)
			runtime.WindowSetAlwaysOnTop(a.ctx, true)
			runtime.WindowSetAlwaysOnTop(a.ctx, false)
		},
		func() { // "Quit"
			os.Exit(0)
		},
		func() { // "Toggle Claude Code Integration"
			a.SetIntegration(!a.GetIntegrationEnabled())
		},
	)
}

// ---- types exposed to JS ----

type CallRecord struct {
	ID        int64  `json:"id"`
	Mode      string `json:"mode"`
	Task      string `json:"task"`
	Result    string `json:"result"`
	Tokens    int    `json:"tokens"`
	LatencyMs int64  `json:"latency_ms"`
	Error     string `json:"error"`
	Time      string `json:"time"`
}

type Stats struct {
	TotalCalls   int   `json:"total_calls"`
	TotalTokens  int   `json:"total_tokens"`
	AvgLatencyMs int64 `json:"avg_latency_ms"`
	SavedTokens  int   `json:"saved_tokens"`
}

type ModelInfo struct {
	Online    bool   `json:"online"`
	ModelName string `json:"model_name"`
}

// ---- Wails-bound methods ----

func (a *App) GetModelInfo() ModelInfo {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	online, name, _ := a.client.ModelStatus(ctx)
	return ModelInfo{Online: online, ModelName: name}
}

func (a *App) GetRecentCalls() []CallRecord {
	calls, _ := a.store.RecentCalls(50)
	out := make([]CallRecord, len(calls))
	for i, c := range calls {
		out[i] = CallRecord{
			ID:        c.ID,
			Mode:      c.Mode,
			Task:      c.Task,
			Result:    c.Result,
			Tokens:    c.Tokens,
			LatencyMs: c.LatencyMs,
			Error:     c.Error,
			Time:      c.CreatedAt.Format("15:04"),
		}
	}
	return out
}

func (a *App) GetIntegrationEnabled() bool {
	val, _ := a.store.GetSetting("claude_integration")
	return val == "true"
}

func (a *App) SetIntegration(enabled bool) error {
	val := "false"
	if enabled {
		val = "true"
	}
	if err := a.store.SetSetting("claude_integration", val); err != nil {
		return err
	}
	tray.UpdateToggle(enabled)
	if enabled {
		return addClaudeIntegration()
	}
	return removeClaudeIntegration()
}

type ActiveTaskInfo struct {
	Mode     string  `json:"mode"`
	Task     string  `json:"task"`
	Progress float64 `json:"progress"`
	Elapsed  int     `json:"elapsed_s"`
}

func (a *App) GetActiveTask() *ActiveTaskInfo {
	t, err := a.store.GetActiveTask()
	if err != nil || t == nil {
		return nil
	}
	// Проверяем что процесс ещё жив
	proc, err := os.FindProcess(t.PID)
	if err != nil || proc.Signal(syscall.Signal(0)) != nil {
		a.store.ClearActiveTask()
		return nil
	}
	return &ActiveTaskInfo{
		Mode:     t.Mode,
		Task:     t.Task,
		Progress: t.Progress,
		Elapsed:  int(time.Since(t.StartedAt).Seconds()),
	}
}

func (a *App) CancelActiveTask() error {
	t, err := a.store.GetActiveTask()
	if err != nil || t == nil {
		return nil
	}
	proc, err := os.FindProcess(t.PID)
	if err != nil {
		return err
	}
	return proc.Signal(syscall.SIGTERM)
}

func (a *App) GetVersion() string {
	return Version
}

func (a *App) GetStats() Stats {
	total, tokens, avgMs, _ := a.store.SessionStats()
	return Stats{
		TotalCalls:   total,
		TotalTokens:  tokens,
		AvgLatencyMs: avgMs,
		SavedTokens:  tokens * 3,
	}
}
