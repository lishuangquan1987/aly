# Server API

aly-server serves a REST API on Gin, unified under the `/api` prefix. No authentication is required (it is designed to run on a trusted intranet).

## Project Management — `/api/project`

| Method | Path | Description |
|---|---|---|
| POST | `/api/project/create_project` | Create a project |
| POST | `/api/project/update_project` | Update project config |
| POST | `/api/project/set_force_update` | Set the force-update flag |
| GET | `/api/project/get_all_projects` | List all projects |
| GET | `/api/project/get_project_by_name/:projectName` | Query a project by name |
| GET | `/api/project/get_project_change_logs/:projectName` | Fetch version changelogs |
| POST | `/api/project/delete_project/:projectName` | Delete a project |
| POST | `/api/project/publish_version` | Publish a version (must be strictly greater than the latest) |
| GET | `/api/project/get_project_os_info/:projectName` | Fetch project OS info |

## Server — `/api/server`

| Method | Path | Description |
|---|---|---|
| GET | `/api/server/info` | Server system info (used by publish-cli `server info`) |

## Files — `/api/file`

| Method | Path | Description |
|---|---|---|
| POST | `/api/file/upload_file` | Upload a file (size-limited) |
| POST | `/api/file/upload_chunk` | Chunked upload (large files) |
| POST | `/api/file/upload_chunk_complete` | Finish a chunked upload (merge & verify) |
| GET | `/api/file/get_all_files/:projectName` | List all files of a project |
| GET | `/api/file/get_release_manifest/:projectName` | Fetch the release manifest (data source for `check_update` / `download_update`) |
| GET | `/api/file/download_file` | Download a file |

## What the Client Actually Touches

One complete client update only hits two endpoints:

1. `GET /api/file/get_release_manifest/:projectName` — `check_update` compares versions; `download_update` reads the file list and hashes.
2. `GET /api/file/download_file` — per-file download.

## Security Notes

- Transfer integrity: downloads are verified with MD5 + SHA256 to guard against corruption.
- Version monotonicity: `publish_version` rejects versions not strictly greater than the latest, preventing accidental stale republishes.
- Upload protection: request body size limit plus a memory multipart cap.
- **Note**: the current version ships no built-in authentication or signing; deploy the server on a trusted network. For public exposure, add authentication at a gateway, or watch the repository's hardening roadmap.
