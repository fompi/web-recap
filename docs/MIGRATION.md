# Rationale & Migration Guide

The CLI interface and database engine of `web-recap` were overhauled to improve usability, prevent data loss, and maintain a clean separation of concerns:

1. **Parameter Simplification (Unified Date/Time)**
   - *Problem:* Previously, users had to combine 5 different parameters (`--date`, `--start-date`, `--end-date`, `--time`, `--start-time`, `--end-time`) to filter dates and times, which was complex and prone to conflicts.
   - *Solution:* Unified all 5 parameter options into exactly two flags: `--from` / `-f` and `--to` / `-t`. They accept full ISO8601 timestamps, dates, or relative helpers (`start` / `0` for start of log, `today`, `yesterday`, `now`, and offsets like `5 hours ago`, `3 days`, `30 minutes`). By default, it extracts history from the beginning of "today" up to "now".
2. **Separation of Concerns via Subcommands**
   - *Problem:* Commands, printing options, and database connection logic were cluttered together on a single root command.
   - *Solution:* Split actions into discrete subcommands:
     - `dump`: Exports history logs (JSON, JSONL, CSV, Table).
     - `stats`: Analyzes user web activity and outputs rich summaries, active browsing durations, transitions, session metrics, and ASCII activity charts.
     - `ingest`: Direct database copy script (keeping connection logic out of the print command).
     - `list`: Helper command to discover active browsers, user profiles, and verify database read permissions (detects macOS Full Disk Access issues).
3. **Multi-Profile Harvesting**
   - *Problem:* Most users have multiple profiles (e.g. Work vs Personal). The tool originally searched only for the "Default" profile.
   - *Solution:* Rewrote browser detection to find all active profiles, extracting and stamping entries with their corresponding `profile` name.
4. **Relational vs Flat layouts**
   - *Problem:* Different browsers record different metrics (e.g., Safari logs redirect origins; Chrome logs transition types and visit durations). Normalizing them into a single table causes data loss or results in a sparse table.
   - *Solution:* Designed two modes via the `--flat` flag:
     - **Relational mode (default / `--flat=false`):** Common columns are stored in a parent `history` table with an auto-incrementing `id` primary key. Browser-specific fields are stored in child tables (e.g. `history_chrome`) linked using `history_id` with `ON DELETE CASCADE`.
     - **Flat mode (`--flat=true`):** Denormalizes everything into flat tables repeating the common fields, avoiding relational constraints.

---

## Migration Guide (Before vs After)

| Goal / Scenario | Before | After |
|---|---|---|
| List profiles | `web-recap list` | `web-recap list` |
| Extract today's logs | `web-recap` | `web-recap dump` |
| Specific browser | `web-recap --browser chrome` | `web-recap dump --browser chrome` |
| Filter by date range | `web-recap --start-date 2025-12-01 --end-date 2025-12-15` | `web-recap dump --from 2025-12-01 --to 2025-12-15` |
| Hours range | `web-recap --date 2025-12-15 --start-time 12:00 --end-time 13:00` | `web-recap dump --from 2025-12-15T12:00:00 --to 2025-12-15T13:00:00` |
| Relative offset | (N/A - manually calculate date strings) | `web-recap dump -f "3 days"` |
| Yesterday to now | (N/A) | `web-recap dump -f yesterday -t now` |
| Show summary charts | (Console summary printed by default) | `web-recap stats` |
| Ingest Chrome history | `web-recap --browser chrome --db-path sqlite://hist.db` | `web-recap ingest -c sqlite://hist.db --browser chrome` |
