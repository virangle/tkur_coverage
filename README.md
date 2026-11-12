# tkur_coverage — измеренные HTML-отчеты ТЕХКОНСУР

> Отдельный исходный компонент. Полная Docker-среда, примеры и испытания: [virangle/tkur_fuzzing](https://github.com/virangle/tkur_fuzzing). Команды ниже с путями `components/`, `examples/` и `tests/` выполняются из корня рекурсивно клонированной среды `tkur_fuzzing`.

Самостоятельная установка: `python3 -m pip install .`; команда `tkur_coverage --help`.


Компонент повторно выполняет **сохраненный корпус**, получает реальные данные LLVM source-based coverage для C/C++ либо coverage.py для Python и выпускает автономный HTML-отчет. Сводка и header/footer русскоязычные; подробные исходные представления LLVM и coverage.py остаются на языке upstream. Полная локализация upstream **не заявляется**.

## Оформление tkur_jacoco

В версии 0.1.1 используется исходный `report.css`, `tkur.svg` и SVG-иконки из [virangle/tkur_jacoco](https://github.com/virangle/tkur_jacoco/tree/fbd7e252d9a424db1e5a7dd685dd25f520a3db15), commit `fbd7e252d9a424db1e5a7dd685dd25f520a3db15`. Header/footer повторяют структуру реального `ReportPage.java`: логотип, полное название «Испытательная лаборатория ООО \"ТЕХКОНСУР\"», название продукта, время `dd.MM.yyyy HH:mm:ss ±HHmm`, breadcrumb, легенда и компактная таблица. Продукт указан честно как `tkur_coverage 0.1.1`; генераторы — LLVM/coverage.py, не JaCoCo. Исходный шрифт Arial / DejaVu Sans 14px и палитра сохранены; код показывается моноширинным шрифтом.

`assets/report.css` и upstream SVG скопированы byte-for-byte, `assets/adapter.css` адаптирует таблицы и source views LLVM/coverage.py к тому же оформлению без изменения счетчиков, строк и интерактивных data attributes. Стандартная keyboard-иконка coverage.py заменена локальной SVG-иконкой с сохранением toggle ID. Из сводки убраны технические поля, карточки и пояснительные блоки; все служебные данные остаются в JSON. Существующие upstream сведения об авторстве не удаляются. Внешние шрифты, CDN и сетевые ресурсы не требуются.

Точное происхождение, git blob SHA и SHA256 записаны в `assets/provenance.json`. Лицензия upstream JaCoCo — EPL-2.0 (`assets/jacoco-LICENSE.md`), иконок — `assets/lucide-LICENSE.txt`; `assets/NOTICE.txt` сохраняет attribution и описание адаптации. Эти файлы входят в wheel и каждый отчет. Лицензии LLVM/coverage.py сохраняют действие. Вспомогательный `fetch_style.py` получает exact bytes через публичный GitHub blob API; сеть нужна только для разработки, не для генерации или просмотра отчетов.

## Установка

В Ubuntu-контейнере `tkur-dev` (все команды далее внутри `/workspace`):

```sh
python3 -m pip install -e components/tkur_coverage
```

Либо без установки: `PYTHONPATH=/workspace/components/tkur_coverage python3 -m tkur_coverage ...`. Основная команда — `tkur_coverage`; `tkur-coverage` оставлена как дополнительный alias. Требуются Python >=3.10, coverage.py 7.x; для native — совместимые clang/llvm-cov/llvm-profdata. В подготовленном окружении используются clang18/LLVM18 и Python3.12.

## C и C++

Отдельный coverage-build нужен **даже если fuzz-build уже имеет sanitizer coverage**:

```sh
clang-18 -O0 -g -fprofile-instr-generate -fcoverage-mapping target.c -o target-c
clang++-18 -O0 -g -fprofile-instr-generate -fcoverage-mapping target.cpp -o target-cpp
```

Бинарь должен принять путь единственного входного файла: `BINARY INPUT`. Для libFuzzer-совместимой цели соберите target с `-fsanitize=fuzzer-no-link -fprofile-instr-generate -fcoverage-mapping`, затем прилинкуйте соответствующую tkur_libfuzzer runtime. Это ABI согласовано с native-компонентом. Готовые fuzz-бинарники без LLVM profile instrumentation для этого отчета непригодны.

```sh
LLVM_COV=llvm-cov-18 LLVM_PROFDATA=llvm-profdata-18 \
python3 -m tkur_coverage native \
  --binary /workspace/build/target-c \
  --corpus /workspace/corpus/c \
  --sources-dir /workspace/examples/c \
  --language C \
  --output /workspace/artifacts/reports/c
```

Для C++ используйте `--language C++` и свой binary/source root. `--binary-arg=-flag=value` можно повторять: аргументы ставятся перед INPUT. По умолчанию timeout каждого процесса 30 секунд, настраивается `--timeout`. Неуспешные replay фиксируются в JSON; нет гарантии, что аварийно завершившийся процесс успел записать `.profraw`. Если нет ни одного профиля, отчет не создается и CLI возвращает 1. Профили успешных и сохранивших профиль неуспешных процессов объединяются; нарушения целостности профилей являются ошибкой, а не поводом подставлять нули.

`--sources-dir` ограничивает отчет реально инструментированными файлами внутри этого дерева. Источники должны оставаться доступны во время генерации; HTML затем содержит строки исходного кода и работает автономно. Для воспроизводимости сохраняются бинарный SHA256, хеши corpus inputs, версии инструментов и исходные LLVM JSON/profiles. Native ABI предусматривает один instrumented binary, не набор динамически загруженных coverage objects.

## Python

Цель: `MODULE:CALLABLE`, где callable принимает `bytes`. Импорт начинается **после** `coverage.start()`, затем каждый regular corpus file передается функции. Уже импортированную цель API отвергает, чтобы не скрывать потерю import-time coverage; предпочтителен отдельный CLI-процесс.

```sh
PYTHONPATH=/workspace/components/tkur_coverage:/workspace/components/tkur_atheris:/workspace/examples/python \
python3 -m tkur_coverage python \
  --target branch_target:TestOneInput \
  --corpus /workspace/examples/python/corpus \
  --source /workspace/examples/python \
  --output /workspace/artifacts/reports/python
```

`--source` — каталог, не dotted module. В него следует включить именно измеряемый код, а не весь workspace/venv. Обычные `Exception` от callback отмечаются в JSON и replay продолжается; `SystemExit`/`KeyboardInterrupt` не подавляются. Цель выполняется в процессе CLI, поэтому бесконечный цикл/`os._exit` не изолируются: запускайте в контейнере с внешним timeout. Компонент не заменяет sandbox. coverage.py измеряет Python statements и branch arcs, а не нативные строки библиотек.

## Артефакты и смысл метрик

Каталог вывода должен быть новым либо пустым и не пересекаться с корпусом. Существующие отчеты и пользовательские данные не перезаписываются. Пустой корпус отвергается. Регулярные файлы собираются рекурсивно; symlink-файлы не воспроизводятся.

- `index.html` — компактная сводная таблица в оформлении tkur_jacoco, время генерации с часовым поясом и навигация к исходным файлам.
- `source/index.html` — upstream line-by-line HTML с добавленными scoped header/footer; original source views, таблицы и attribution сохранены.
- `report.json` — схема v1: `measurement: corpus-replay-source-coverage`, `fuzzing_feedback: false`, `metrics`, SHA256/размер входов, список исходников/target, результаты каждого replay, версии и дата.
- `source-files/*` — точные байты измеренных исходников; исходный путь, относительный путь снимка и SHA256 записаны в `report.json`. Это не переформатированные копии HTML.
- Native: `llvm-export.json`, `profiles/*.profraw`, `profiles/merged.profdata`.
- Python: `coverage.json`, `.coverage`; скопированные offline CSS/JS coverage.py.

Проценты сводки = `100 * covered / total` из соответствующего engine JSON, округлены до 2 знаков. Нулевой знаменатель отображается как «н/д», не как фиктивные 100%. LLVM line coverage и Python statement coverage имеют разные определения; ветви LLVM и coverage.py также нельзя сравнивать напрямую.

**Онлайн `cov`/`ft` фаззера не являются процентами покрытия исходных строк.** Эти отчеты измеряют только сохраненный корпус при конкретном replay, не все входы, когда-либо исполненные фаззером. `report.json` не выдумывает fuzzing feedback, а явно устанавливает `fuzzing_feedback: false`. Время генерации — не время всей fuzz-сессии. Наличие report не означает отсутствие ошибок цели: смотрите список `replay` в JSON.

## Проверка реальными инструментами

```sh
LLVM_COV=llvm-cov-18 LLVM_PROFDATA=llvm-profdata-18 \
TKUR_REPORT_ARTIFACTS=/workspace/artifacts/coverage-fixtures \
python3 -m pytest -q tests/coverage
```

Тесты действительно компилируют и запускают C/C++ ветвящиеся targets и запускают Python target в свежем процессе, сравнивают все сводные метрики с raw JSON, проверяют известные missing lines, HTML escaping, локальные ссылки, branding, сохранение исходного attribution и отсутствие внешних assets. При отсутствии toolchain тест native помечается skip, а не выдается за успешное измерение. Указанный каталог сохранения fixture reports должен быть новым; повторный запуск используйте с другим каталогом.

## Лицензии

Код адаптации оформления и генератора распространяется по EPL-2.0; см. LICENSE. Оригинальные CSS/шаблоны JaCoCo — EPL-2.0; SVG-иконки имеют собственные ISC/MIT notices. Логотипы не перелицензируются. Полные notices находятся в tkur_coverage/assets/.

Для validate_package.py сначала создайте wheel: `python -m build --wheel --outdir wheel-check`.
