/// MiraBase context-aware client views (the `mirabase` Dart package).
///
/// Context is immutable per scoped client instance (api-and-sdk.md):
/// "simultaneous calls cannot race through a shared currentDatabase
/// property." A scoped view sends the resolved selector on every request
/// via the X-MiraBase-Database / X-MiraBase-Tenant headers; invalid or
/// unauthorized selection never falls back to primary on the server, and
/// the view never rewrites itself after creation.
library mirabase.context;

import 'dart:convert';
import 'dart:io';

/// Low-level transport shared by all views.
class MirabaseClient {
  MirabaseClient(this.baseUrl, {String? Function()? tokenProvider})
      : _tokenProvider = tokenProvider ?? (() => null) {
    baseUrl = baseUrl.replaceAll(RegExp(r'/+$'), '');
  }

  String baseUrl;
  String? Function()? _tokenProvider;

  /// withContext returns an immutable database-scoped view.
  MirabaseContextView withContext({required String databaseId}) =>
      MirabaseContextView(this, databaseId);

  /// withTenant returns an immutable tenant-scoped view.
  MirabaseTenantView withTenant(
          {required String namespace, required String key}) =>
      MirabaseTenantView(this, namespace, key);

  void setTokenProvider(String? Function()? provider) {
    _tokenProvider = provider;
  }

  Future<dynamic> request(
    String path, {
    String method = 'GET',
    Object? body,
    Map<String, String> query = const {},
    Map<String, String> extraHeaders = const {},
  }) async {
    if (!path.startsWith('/') || path.startsWith('//')) {
      throw ArgumentError('request path must begin with a single slash');
    }
    final uri = Uri.parse(baseUrl + path);
    final base = Uri.parse(baseUrl);
    final prefix = base.path.replaceAll(RegExp(r'/+$'), '');
    if (uri.origin != base.origin ||
        (prefix.isNotEmpty && !uri.path.startsWith('$prefix/'))) {
      throw ArgumentError('request path escaped its application URL');
    }
    final effective = Uri(
      scheme: uri.scheme,
      host: uri.host,
      port: uri.hasPort ? uri.port : null,
      path: uri.path,
      queryParameters: query.isEmpty ? uri.queryParameters : query,
    );
    final headers = <String, String>{
      'Accept': 'application/json',
      ...extraHeaders,
    };
    final token = _tokenProvider?.call();
    if (token != null) {
      headers['Authorization'] = token;
    }
    if (body != null && !headers.containsKey('Content-Type')) {
      headers['Content-Type'] = 'application/json';
    }
    final client = HttpClient();
    try {
      final req = await client.openUrl(method, effective);
      headers.forEach(req.headers.set);
      if (body != null) {
        final encoded = jsonEncode(body);
        req.add(utf8.encode(encoded));
      }
      final resp = await req.close();
      final text = await resp.transform(utf8.decoder).join();
      dynamic decoded;
      if (text.isNotEmpty) {
        try {
          decoded = jsonDecode(text);
        } catch (_) {
          decoded = text;
        }
      }
      if (resp.statusCode >= 400) {
        final dynamic data = decoded;
        final code = (data is Map &&
                data['data'] is Map &&
                data['data']['mirabase'] is Map)
            ? data['data']['mirabase']['code'] as String
            : 'invalid_request';
        throw MirabaseException(
          code: code,
          status: resp.statusCode,
          message: data is Map && data['message'] is String
              ? data['message'] as String
              : 'request failed (${resp.statusCode})',
          response: data,
        );
      }
      return decoded;
    } finally {
      client.close();
    }
  }
}

/// Structured additive error envelope without string parsing.
class MirabaseException implements Exception {
  MirabaseException({
    required this.code,
    required this.status,
    required this.message,
    this.response,
  });

  final String code;
  final int status;
  final String message;
  final dynamic response;

  @override
  String toString() => 'MirabaseException($code, $status): $message';
}

/// Immutable database-scoped view (X-MiraBase-Database).
class MirabaseContextView {
  MirabaseContextView(this._client, this.databaseId);

  final MirabaseClient _client;
  final String databaseId;

  Future<dynamic> request(String path,
      {String method = 'GET', Object? body, Map<String, String> query = const {}}) {
    return _client.request(path,
        method: method,
        body: body,
        query: query,
        extraHeaders: {'X-Mirabase-Database': databaseId});
  }

  /// Context-bound record CRUD on /api/mirabase/v1.
  Future<dynamic> listRecords(String collection, {int limit = 100}) =>
      request(
          '/api/mirabase/v1/db/$databaseId/collections/$collection/records',
          query: {'limit': '$limit'});

  Future<dynamic> getRecord(String collection, String recordId) => request(
      '/api/mirabase/v1/db/$databaseId/collections/$collection/records/$recordId');

  Future<dynamic> createRecord(
          String collection, Map<String, dynamic> data) =>
      request(
          '/api/mirabase/v1/db/$databaseId/collections/$collection/records',
          method: 'POST',
          body: data);

  Future<dynamic> updateRecord(
          String collection, String recordId, Map<String, dynamic> data) =>
      request(
          '/api/mirabase/v1/db/$databaseId/collections/$collection/records/$recordId',
          method: 'PATCH',
          body: data);

  Future<void> deleteRecord(String collection, String recordId) async {
    await request(
        '/api/mirabase/v1/db/$databaseId/collections/$collection/records/$recordId',
        method: 'DELETE');
  }
}

/// Immutable tenant-scoped view (X-MiraBase-Tenant: namespace/key).
class MirabaseTenantView {
  MirabaseTenantView(this._client, this.namespace, this.key);

  final MirabaseClient _client;
  final String namespace;
  final String key;

  Future<dynamic> request(String path,
      {String method = 'GET', Object? body, Map<String, String> query = const {}}) {
    return _client.request(path,
        method: method,
        body: body,
        query: query,
        extraHeaders: {'X-Mirabase-Tenant': '$namespace/$key'});
  }
}

/// forApplication — the immutable entry shape.
MirabaseClient forApplication(String baseUrl,
        {String? Function()? tokenProvider}) =>
    MirabaseClient(baseUrl, tokenProvider: tokenProvider);
