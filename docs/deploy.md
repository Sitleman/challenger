# Деплой и восстановление

Автоматические stage/prod-выкладки и их включение описаны в [релизах](releases.md).
Ниже — прежняя ручная процедура и восстановление исходного окружения. После
включения CI/CD обновлять версии через workflows, чтобы не рассинхронизировать
серверное состояние и changelog.

## Рабочее окружение

С 3 ноября 2026 года бот работает на новом сервере:

| Параметр | Значение |
|---|---|
| SSH | `ssh root@202.155.9.90` |
| ОС | Ubuntu 24.04 |
| Репозиторий | `/opt/challenger`, ветка `master` |
| Контейнер / образ | `challenger-bot-1` / `challenger-bot` |
| Том / БД | `challenger-data` / `/data/challenger.db` |
| Конфигурация | `/opt/challenger/.env`, права `600` |
| Перезапуск | Docker включён в systemd; контейнер — `unless-stopped` |

Команды ниже выполняются под root в `/opt/challenger`. Docker, Compose и Buildx
установлены пакетами Ubuntu: `docker.io`, `docker-compose-v2`, `docker-buildx`.
Старый сервер `158.160.144.23` больше не используется.

На новом сервере Ubuntu 24.04 установить Docker и включить его автозапуск:

```bash
set -eu
apt-get update
apt-get install -y docker.io docker-compose-v2 docker-buildx
systemctl enable --now docker
```

`BOT_TOKEN` хранится в `.env`; значение не коммитить и не выводить в логи команд.
В [docker-compose.yml](../docker-compose.yml) заданы:

- `ALLOWED_CHAT_IDS`: `-1003436893374` — основной чат Monthly challenges,
  `-1004391088785` — stage.
- `ADMIN_USER_IDS`: `532029049`.
- `DB_PATH`: `/data/challenger.db`.

Для локального `make run` эти переменные берутся из `.env`.
Часовой пояс `CHALLENGE_TZ` по умолчанию — `Europe/Moscow`.

Commit работающей версии хранится отдельно от checkout, в
`/root/challenger-backups/deployed.commit`. Перед первым обновлением файл
нужно один раз заполнить commit, из которого действительно собран запущенный
образ. Для версии, перенесённой 3 ноября, это `c05cae0c4e4306722e42b9d99c7fafe409876bcb`:

```bash
set -eu
backup_dir=/root/challenger-backups
install -d -m 700 "$backup_dir"
test ! -e "$backup_dir/deployed.commit"
printf '%s\n' c05cae0c4e4306722e42b9d99c7fafe409876bcb > "$backup_dir/deployed.commit"
```

Если после переноса уже выкатывали другую версию, указать её проверенный
commit. Не подставлять текущий `HEAD`: после отката или неудачной сборки он
может отличаться от версии работающего контейнера.

## Обновление

Сначала проверить `git status --short`: чужие изменения не сбрасывать.
Новый образ собирается при работающем боте. Остановка нужна для согласованного
бэкапа тома и переключения на новую версию.

```bash
set -eu
cd /opt/challenger
test "$(git branch --show-current)" = master
git diff --quiet
git diff --cached --quiet
deploy_stamp=$(date -u +%Y%m%dT%H%M%SZ)
backup_dir=/root/challenger-backups
install -d -m 700 "$backup_dir"
previous_commit=$(cat "$backup_dir/deployed.commit")
git cat-file -e "$previous_commit^{commit}"
rollback_tag="challenger-bot:before-$deploy_stamp"
previous_image=$(docker inspect challenger-bot-1 --format '{{.Image}}')
docker image tag "$previous_image" "$rollback_tag"
printf '%s\n' "$previous_commit" > "$backup_dir/$deploy_stamp.commit"
printf '%s\n' "$rollback_tag" > "$backup_dir/$deploy_stamp.image"
git pull --ff-only
target_commit=$(git rev-parse HEAD)
docker compose build bot
docker compose stop bot
volume_dir=$(docker volume inspect challenger-data --format '{{.Mountpoint}}')
backup_file="$backup_dir/$deploy_stamp.tgz"
tar czf "$backup_file" -C "$volume_dir" .
tar tzf "$backup_file" >/dev/null
docker compose up -d --no-build bot </dev/null
printf 'Rollback: %s\nBackup: %s\n' "$deploy_stamp" "$backup_file"
```

Если сборка не удалась, прежний контейнер продолжает работать. Если ошибка
возникла после остановки, до переключения на новый контейнер, прежний можно
запустить командой `docker start challenger-bot-1`.
При продолжении неудавшегося обновления сохранить исходные `previous_commit`
и `rollback_tag` из файлов в `backup_dir`: checkout уже мог обновиться, хотя
контейнер ещё работает на старом образе.
После запуска выполнить [проверку](#проверка-запуска) и только после её успеха
записать `target_commit` в файл работающей версии. Если новая версия не прошла
проверку, сначала выполнить откат; не начинать очередное обновление с
неподтверждённым контейнером.

На сервере 1 CPU и 1 ГБ RAM. При полной сборке может понадобиться временный swap.
Следующий блок применять только при отсутствии swap и такого файла:

```bash
set -eu
test -z "$(swapon --noheadings --show=NAME)"
test ! -e /var/tmp/challenger-build.swap
fallocate -l 2G /var/tmp/challenger-build.swap
chmod 600 /var/tmp/challenger-build.swap
mkswap /var/tmp/challenger-build.swap
swapon /var/tmp/challenger-build.swap
```

После окончания сборки отключить и удалить созданный файл:

```bash
set -eu
swapoff /var/tmp/challenger-build.swap
rm /var/tmp/challenger-build.swap
```

## Восстановление БД

Использовать файл, присланный ботом в Telegram ежедневно в 04:00 или по
`/make_backup` в личке администратора. Это самодостаточный снимок SQLite;
обычная копия работающей WAL-базы без `-wal` может потерять последние записи.
Механизм описан в [ADR 0003](decisions/0003-backup-telegram.md).

При переносе 3 ноября использован файл
`/root/challenger-bootstrap/challenger-20261102-0400.db`. Перед следующим
восстановлением указать нужный файл в `restore_src`; исходник сохраняется.

Образ запускается с UID/GID `65532:65532`. Файл с правами `600`, принадлежащий
root, контейнер прочитать не сможет, поэтому для `restore` создаётся отдельная
доступная ему копия и монтируется только этот файл.

```bash
set -eu
cd /opt/challenger
restore_src=/root/challenger-bootstrap/challenger-20261102-0400.db
restore_dir=$(mktemp -d /tmp/challenger-restore.XXXXXX)
install -o 65532 -g 65532 -m 400 "$restore_src" "$restore_dir/backup.db"
docker compose stop bot
docker compose run --rm --no-deps -T \
  -v "$restore_dir/backup.db:/restore.db:ro" bot restore /restore.db </dev/null
rm "$restore_dir/backup.db"
rmdir "$restore_dir"
docker compose up -d --no-build bot </dev/null
```

Для нового сервера сначала клонировать репозиторий, заполнить `.env` из
`.env.example`, выставить `chmod 600 .env` и собрать образ. Первый запуск с
`up -d` выполнять после восстановления, чтобы бот не начал работу с пустой БД.

`restore` проверяет целостность, совместимость схемы и блокировку работающего
бота. Прежняя БД откладывается в `/data/challenger.db.before-restore-<время>`;
миграции восстановленной БД применяются при запуске основного контейнера.
Если восстановление не удалось, основной бот остаётся остановленным до
выяснения причины. Перенаправление `</dev/null` не даёт `compose run` забрать
оставшиеся команды при запуске через `ssh ... 'bash -s'`.

## Откат обновления

Выбрать метку `deploy_stamp`, напечатанную при обновлении. Образ, commit и
бэкап должны относиться к одному обновлению. После возврата БД изменения,
сделанные с момента бэкапа, останутся только в отложенном состоянии тома.

```bash
set -eu
cd /opt/challenger
deploy_stamp=20261103T194000Z
backup_dir=/root/challenger-backups
previous_commit=$(cat "$backup_dir/$deploy_stamp.commit")
target_commit=$previous_commit
rollback_tag=$(cat "$backup_dir/$deploy_stamp.image")
backup_file="$backup_dir/$deploy_stamp.tgz"
docker image inspect "$rollback_tag" >/dev/null
tar tzf "$backup_file" >/dev/null
git diff --quiet
git diff --cached --quiet
docker compose stop bot
git switch --detach "$previous_commit"
volume_dir=$(docker volume inspect challenger-data --format '{{.Mountpoint}}')
aside_dir="$backup_dir/failed-$(date -u +%Y%m%dT%H%M%SZ)"
install -d -m 700 "$aside_dir"
find "$volume_dir" -mindepth 1 -maxdepth 1 -exec mv -t "$aside_dir" -- {} +
tar xzf "$backup_file" -C "$volume_dir"
docker image tag "$rollback_tag" challenger-bot:latest
docker compose up -d --no-build bot </dev/null
```

Откат возвращает образ и БД вместе: старая версия может не поддерживать
миграции новой. Текущее содержимое тома переносится в `aside_dir`, включая
`-wal`/`-shm`, а не смешивается с восстановленными файлами.
После [проверки](#проверка-запуска) сохранить `target_commit` в файле работающей
версии тем же способом, что и при обновлении. Затем можно вернуться на `master`
командой `git switch master`: checkout изменится, но следующий откат возьмёт
commit из `deployed.commit`, соответствующий работающему образу и БД.

## Проверка запуска

```bash
set -euo pipefail
cd /opt/challenger
systemctl is-enabled docker
docker compose ps
docker inspect challenger-bot-1 \
  --format 'status={{.State.Status}} restarts={{.RestartCount}} restart_policy={{.HostConfig.RestartPolicy.Name}}'
docker compose logs --since 5m --no-color bot 2>&1 \
  | sed -E 's/[0-9]{6,}:[A-Za-z0-9_-]{20,}/[REDACTED]/g; /API call to:/d; s/, Result:.*$//' \
  | grep -E 'bot @.* started|API response (getUpdates|editMessageText): Ok:|API response [[:alnum:]_]+: Ok: false|get updates:|getMe:|migrate up:|goose provider:|open sqlite:|database is in use|Execution error|Error on get me|panic:|fatal error:|another bot instance is polling|render stats for challenge'
```

Ожидаются `running`, отсутствие перезапусков, строка
`bot @month_challenger_bot ... started` и успешные ответы `getUpdates`.
Первый ответ пустого long polling может занять 30 секунд. Обновление
закреплённой статистики видно по `editMessageText`; ответ `message is not modified`
означает, что текст уже актуален.

Ошибки `get updates:`, `getMe:`, миграции, SQLite и выполнения API-запросов
также остаются в выводе. Тела ответов и строки запросов удаляются до фильтрации,
чтобы текст пользовательского сообщения не попал в вывод из-за совпадения с
шаблоном ошибки. Состояния `running` недостаточно: пока нет успешного
`getUpdates` или есть неустранённые ошибки, запуск не подтверждён и
`deployed.commit` не менять. Ожидаемый `message is not modified` ошибкой запуска
не считается.

После успешной проверки обновления или отката сохранить `target_commit` из
соответствующего блока выше. Если shell-сессия прервалась, сначала восстановить
его значение по проверенной версии контейнера. Запись заменяется атомарно:

```bash
set -eu
test -n "$target_commit"
backup_dir=/root/challenger-backups
printf '%s\n' "$target_commit" > "$backup_dir/deployed.commit.next"
mv "$backup_dir/deployed.commit.next" "$backup_dir/deployed.commit"
```

Сырые debug-логи содержат URL с токеном и сообщения пользователей, поэтому
перед публикацией использовать фильтрацию. При конфликте второго экземпляра
бот перестаёт обрабатывать обновления: остановить лишний экземпляр и
перезапустить нужный, как описано в [README](../README.md).
