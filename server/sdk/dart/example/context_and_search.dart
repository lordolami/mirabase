/// MiraBase runnable example: context-bound records through the actual
/// /api/mirabase/v1 surface.
///
/// Run against a real backend:
///   MIRABASE_URL=http://127.0.0.1:8090 \
///   MIRABASE_TOKEN=<record-token> \
///   MIRABASE_DATABASE_ID=<database-id> \
///   dart run example/context_and_search.dart
import 'dart:io';

import 'package:mirabase/mirabase.dart';

void main(List<String> args) async {
  final base = Platform.environment['MIRABASE_URL'] ?? 'http://127.0.0.1:8090';
  final token = Platform.environment['MIRABASE_TOKEN'];
  final databaseId = Platform.environment['MIRABASE_DATABASE_ID'];
  if (token == null || databaseId == null) {
    stderr.writeln('MIRABASE_TOKEN and MIRABASE_DATABASE_ID are required');
    exitCode = 1;
    return;
  }

  final client = forApplication(base, tokenProvider: () => token);
  final db = client.withContext(databaseId: databaseId);

  // 1. Create a record in the scoped context.
  final created =
      await db.createRecord('notes', {'text': 'example record'});
  stdout.writeln('created: ${created['id']}');

  // 2. Read it back.
  final read = await db.getRecord('notes', created['id'] as String);
  if (read['text'] != 'example record') {
    stderr.writeln('read-back mismatch');
    exitCode = 1;
    return;
  }

  // 3. Text search with the index maintenance fallback.
  Future<dynamic> search() => db.request(
      '/api/mirabase/v1/databases/$databaseId/collections/notes/search',
      method: 'POST',
      body: {
        'mode': 'text',
        'text': {'fields': ['text'], 'query': 'example'}
      });
  try {
    final hits = await search();
    stdout.writeln('text search hits: ${(hits['items'] as List).length}');
  } on MirabaseException catch (err) {
    if (err.code != 'schema_mismatch') rethrow;
    // The FTS index does not exist yet — create it (durable maintenance).
    await db.request(
        '/api/mirabase/v1/databases/$databaseId/collections/notes/search-index',
        method: 'POST',
        body: {
          'indexName': 'idx_notes_fts',
          'fields': ['text'],
          'tokenizer': 'default'
        });
    final hits = await search();
    stdout.writeln(
        'text search hits (after index creation): ${(hits['items'] as List).length}');
  }

  // 4. Release-capability error: hybrid must return capability_unavailable.
  try {
    await db.request(
        '/api/mirabase/v1/databases/$databaseId/collections/notes/search',
        method: 'POST',
        body: {'mode': 'hybrid'});
    stderr.writeln('hybrid was accepted (must not be)');
    exitCode = 1;
    return;
  } on MirabaseException catch (err) {
    if (err.code != 'capability_unavailable') rethrow;
    stdout.writeln('hybrid correctly rejected: capability_unavailable');
  }

  // 5. Cleanup.
  await db.deleteRecord('notes', created['id'] as String);
  stdout.writeln('deleted: ${created['id']}');
  stdout.writeln('EXAMPLE OK');
}
