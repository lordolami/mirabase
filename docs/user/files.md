# Files

File fields store uploaded files through the configured filesystem backend.
MiraBase generates storage names, preserves the original filename metadata,
supports thumbnails for supported image types, and applies collection access
rules to downloads.

## Uploads

Uploads use multipart record create/update requests. File size, count, MIME
type, and extension limits come from the field definition. Uploaded executable
or active content must never be executed by file preview or download handling.

## Protected files

Protected files require an authorized file token or a request context allowed
by the collection rules. A storage path alone is not authorization. Signed or
tokenized access must expire and must remain bound to the intended file and
application context.

## Storage maintenance

Backups include file storage according to the backup configuration. When a
record or file field value is removed, orphan cleanup follows the configured
filesystem behavior and must not delete files still referenced by other
records.
