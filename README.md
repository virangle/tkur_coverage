# tkur_coverage — измеренные HTML-отчеты

Самостоятельный генератор отчетов **Испытательной лаборатории ООО «ТЕХКОНСУР»**. Генерация не требует общего репозитория, Docker, Atheris или запущенного фаззера.

Доступны два самостоятельных интерфейса:

- **Native 0.3.0** — готовый Linux x86_64 executable без Python для импорта измеренного Rust/LLVM, Go, JavaScript/TypeScript (Istanbul JSON) и C#/.NET (Coverlet JSON) coverage. Сам не запускает цели. Сохранены backend-команды 0.2.0. [Установка, сборка и команды native](native/README.md).
- **Python 0.1.1** — сохраненный совместимый wheel: повторное выполнение корпуса для C/C++ (LLVM) и Python (coverage.py). Его API и версия не изменены; инструкции ниже относятся к нему.

Оба варианта используют одинаковые проверенные assets и оформление. Команда у них называется `tkur_coverage`: выберите нужный бинарник в PATH либо используйте полный путь; Python-вариант всегда доступен как `python3 -m tkur_coverage`. Native release содержит только compiled executable и обязательные лицензии, без исходников/скриптов/Docker.

## Установка из release wheel

Требуется Python >=3.10. Для C/C++ дополнительно нужны согласованные версии clang, llvm-cov и llvm-profdata; проверенная конфигурация — LLVM18 и Python3.12. Зависимость Python — coverage.py7.x.

Скачайте wheel из [Releases](https://github.com/virangle/tkur_coverage/releases), затем:

```sh
python3 -m venv .venv
. .venv/bin/activate
python3 -m pip install ./tkur_coverage-0.1.1-py3-none-any.whl
tkur_coverage --help
```

Установка coverage.py выполняется pip как зависимости. Для полностью offline-установки заранее скачайте также совместимый coverage.py wheel и используйте `pip install --no-index --find-links WHEELS_DIR tkur_coverage==0.1.1`. Сам просмотр отчетов не требует сети.

Из исходного репозитория компонент устанавливается командой `python3 -m pip install .` из его корня. Основная команда — `tkur_coverage`; `python3 -m tkur_coverage` и alias `tkur-coverage` эквивалентны. Release распространяет wheel, не Docker-образ и не архив рабочего окружения. Примеры ниже можно создать локально; в исходном репозитории те же targets доступны в каталоге `examples`.

## Быстрый старт без исходного репозитория

Команды предназначены для POSIX shell. Создайте рабочий каталог вне установленного пакета:

```sh
mkdir -p coverage-demo/native coverage-demo/python coverage-demo/corpus coverage-demo/build
cd coverage-demo
printf A > corpus/A
printf B > corpus/B
```

### C и C++: бинарь с ABI `BINARY INPUT`

Создайте `native/target.c`:

```c
#include <stdio.h>
static int classify(int byte) {
    if (byte == 'A') return 10;
    if (byte == 'B') return 20;
    return 30;
}
int main(int argc, char **argv) {
    if (argc != 2) return 2;
    FILE *input = fopen(argv[1], "rb");
    if (!input) return 3;
    int byte = fgetc(input);
    fclose(input);
    return classify(byte) == 999;
}
```

Создайте **отдельный coverage-build**, даже если fuzz-build имеет sanitizer coverage:

```sh
clang-18 -O0 -g -fprofile-instr-generate -fcoverage-mapping native/target.c -o build/target-c
clang++-18 -x c++ -O0 -g -fprofile-instr-generate -fcoverage-mapping native/target.c -o build/target-cpp

LLVM_COV=llvm-cov-18 LLVM_PROFDATA=llvm-profdata-18 \
tkur_coverage native --binary build/target-c --corpus corpus \
  --sources-dir native --language C --output reports/c

LLVM_COV=llvm-cov-18 LLVM_PROFDATA=llvm-profdata-18 \
tkur_coverage native --binary build/target-cpp --corpus corpus \
  --sources-dir native --language C++ --output reports/cpp
```

Результат — `reports/c/index.html` и `reports/cpp/index.html`. Оба бинаря принимают путь одного файла и завершаются после его обработки. LibFuzzer-compatible binaries также подходят, если собраны с `-fprofile-instr-generate -fcoverage-mapping` и поддерживают replay одного INPUT; сам генератор не требует конкретной fuzzing runtime. Обычный fuzz-binary без profile instrumentation непригоден.

`--binary-arg=-flag=value` можно повторять: аргументы ставятся перед INPUT. Timeout каждого процесса по умолчанию 30 секунд, настраивается положительным конечным `--timeout`. Неуспешный replay фиксируется в JSON; аварийно завершившийся процесс может не успеть сохранить профиль. Если нет ни одного LLVM-профиля, CLI возвращает 1, не подставляя фиктивные метрики. Некорректный профиль является ошибкой.

`--sources-dir` ограничивает отчет инструментированными файлами внутри заданного дерева. Исходники должны оставаться доступны во время генерации; затем HTML содержит строки и открывается автономно. Поддерживается один instrumented binary, не набор динамически загружаемых coverage objects.

### Python: callback принимает `bytes`

Создайте `python/target.py`:

```python
def TestOneInput(data):
    if data == b"A":
        result = 10
    elif data == b"B":
        result = 20
    else:
        result = 30
    return result
```

Запуск из каталога `coverage-demo`:

```sh
PYTHONPATH="$PWD/python" tkur_coverage python --target target:TestOneInput \
  --corpus corpus --source python --output reports/python
```

Результат — `reports/python/index.html`. Импорт начинается после `coverage.start()`, затем каждый вход передается callback. Уже импортированный target API отвергает: используйте отдельный CLI-процесс для честного import-time coverage. `--source` — каталог, не dotted module; не указывайте весь venv.

Обычные `Exception` от callback записываются в JSON, replay продолжается. `SystemExit`/`KeyboardInterrupt` не подавляются. Python-target выполняется в процессе CLI: бесконечный цикл и `os._exit` не изолируются. Для недоверенных целей нужен внешний sandbox и timeout, например `timeout 60s env PYTHONPATH="$PWD/python" tkur_coverage python ...`. Компонент не заменяет sandbox и измеряет Python statements/branch arcs, не нативные строки библиотек.

## Данные и ограничения

Каталог вывода должен быть новым либо пустым и не пересекаться с корпусом; пользовательские файлы и существующие отчеты не перезаписываются. Пустой корпус отвергается. Регулярные файлы собираются рекурсивно; symlink-файлы пропускаются.

- `index.html` — компактная измеренная сводка и навигация.
- `source/index.html` — подробные исходные представления LLVM/coverage.py, оформленные в общем стиле.
- `report.json` — схемаv1, `measurement: corpus-replay-source-coverage`, `fuzzing_feedback: false`, версии инструментов, времяUTC, метрики, SHA256/размер входов и результаты каждого replay.
- `source-files/*` — точные байты измеренных исходников; исходные пути, SHA256 и пути снимков находятся в JSON.
- Native: `llvm-export.json`, `profiles/*.profraw`, `profiles/merged.profdata`, SHA256 бинаря в метаданных.
- Python: `coverage.json`, `.coverage`, локальные CSS/JS.

Проценты сводки = `100 * covered / total`, округлены до 2 знаков. При нулевом знаменателе показано «н/д», не 100%. Для Python quickstart известный результат — 6/7 statements и 3/4 branches; он проверяется по raw JSON, а не подставляется в отчет.

**Онлайн `cov`/`ft` фаззера — не проценты покрытия строк.** Отчет измеряет только сохраненный корпус при replay, не все когда-либо выполненные входы. LLVM lines и Python statements, а также определения branch coverage различаются и не являются межъязыковым рейтингом. Наличие отчета не гарантирует отсутствие ошибок: смотрите `replay` в JSON.

## Оформление и лицензии

Стиль основан на реальных `report.css`, `tkur.svg`, SVG-иконках и структуре `ReportPage.java` из [virangle/tkur_jacoco](https://github.com/virangle/tkur_jacoco/tree/fbd7e252d9a424db1e5a7dd685dd25f520a3db15), pinned commit `fbd7e252d9a424db1e5a7dd685dd25f520a3db15`. Исходные CSS/SVG сохранены byte-for-byte; адаптер оформляет таблицы и source views без изменения данных. Шрифт Arial / DejaVu Sans 14px, код моноширинный. Header содержит логотип, полное название лаборатории, честное название `tkur_coverage 0.1.1` и время с часовым поясом. Внешние assets/CDN не используются.

Сводка и оболочка русскоязычные; полная локализация upstream LLVM/coverage.py не заявляется. Реальные copyright/attribution не удаляются. Git blob SHA, SHA256 и пути происхождения находятся в `tkur_coverage/assets/provenance.json`; они копируются в каждый отчет.

Код генератора и адаптации — EPL-2.0, см.`LICENSE`. Оригинальные JaCoCo CSS/шаблоны — EPL-2.0; иконки — ISC/MIT по их notices. Логотип не перелицензируется. `tkur_coverage/assets/jacoco-LICENSE.md`, `lucide-LICENSE.txt`, `NOTICE.txt` входят в wheel и каждый отчет; лицензии LLVM/coverage.py сохраняют действие.

## Сборка и проверка wheel разработчиком

Из корня данного исходного репозитория:

```sh
python3 -m pip wheel --no-deps . --wheel-dir wheel-check
python3 validate_package.py
```

Проверка подтверждает наличие и byte-for-byte точность обязательных стилевых ресурсов и notices в wheel. Для функционального smoke test выполните standalone quickstart выше: сравните `report.json` с `llvm-export.json`/`coverage.json`, проверьте открытие source pages и известную невыполненную ветку для входов A/B. Репозиторий не требует отсутствующей внешней тестовой среды.

Также доступен `smoke_standalone.py`: запустите его установленным из wheel Python из каталога вне исходного дерева. Он использует только локальные `examples`, компилирует C/C++ цели, запускает Python callback и сравнивает метрики с настоящими engine JSON. Результаты сохраняются в `standalone-check` рядом со скриптом; этот каталог должен отсутствовать перед первым запуском либо иметь новые имена отчетов при повторении. Скрипт намеренно отвергает импорт пакета прямо из исходного дерева, чтобы не выдавать source-tree проверку за проверку wheel.
