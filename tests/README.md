# Интеграционные тесты

Сценарии пользователя против настоящего стека в докере, только через gateway. Как устроено и зачем —
[docs/testing/TEST_PLAN.md](../docs/testing/TEST_PLAN.md); кейсы по сценариям — [docs/testing/flows/](../docs/testing/flows/);
что не сошлось с ожиданиями — [docs/testing/DISCREPANCIES.md](../docs/testing/DISCREPANCIES.md).

```sh
make test-integration          # из корня репозитория: стенд → тесты → стенд погашен
make stand-up && make test-integration-run RUN=TestF01   # быстрый цикл при разработке тестов
```

Пакеты: `integration/` — сами сценарии; `client/` — «искусственный пользователь»; `fakes/` — двойники LLM и
arXiv; `fixtures/` — PDF с известным содержимым. Бэкенд этот модуль не импортирует.
