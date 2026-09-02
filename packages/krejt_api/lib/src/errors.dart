import 'package:dio/dio.dart';

/// Gabimi i API-së në formatin e vetëm të serverit (§55, §57):
/// `{ error: { code, message_key, http_status, request_id, trace_id, retryable, fields } }`.
/// Aplikacioni tregon tekst të përkthyer nga `message_key`, kurrë tekstin e papërpunuar të serverit.
class ApiError implements Exception {
  ApiError({
    required this.code,
    required this.messageKey,
    required this.status,
    this.requestId,
    this.traceId,
    this.retryable = false,
    this.fields = const {},
  });

  final String code;
  final String messageKey;
  final int status;
  final String? requestId;
  final String? traceId;
  final bool retryable;

  /// Gabimet për fushë, për shfaqje inline te formularët: `{"email": "invalid"}`.
  final Map<String, String> fields;

  bool get isUnauthorized => status == 401 || code == 'UNAUTHORIZED' || code == 'SESSION_INVALID';
  bool get isForbidden => status == 403;
  bool get isNotFound => status == 404;
  bool get isValidation => status == 422 || code == 'VALIDATION_FAILED';
  bool get isRateLimited => status == 429;
  bool get needsUpdate => status == 426 || code == 'UPDATE_REQUIRED';
  bool get isMaintenance => code == 'MAINTENANCE';
  bool get isOffline => code == 'OFFLINE';

  static ApiError offline() =>
      ApiError(code: 'OFFLINE', messageKey: 'errors.offline', status: 0, retryable: true);

  static ApiError unknown([Object? cause]) =>
      ApiError(code: 'INTERNAL', messageKey: 'errors.internal', status: 0, retryable: true);

  /// Ndërton gabimin nga përgjigjja e serverit; çdo formë tjetër bëhet INTERNAL pa detaje teknike.
  factory ApiError.fromResponse(Response<dynamic>? res) {
    final data = res?.data;
    if (data is Map && data['error'] is Map) {
      final e = Map<String, dynamic>.from(data['error'] as Map);
      final rawFields = e['fields'];
      return ApiError(
        code: (e['code'] ?? 'INTERNAL').toString(),
        messageKey: (e['message_key'] ?? 'errors.internal').toString(),
        status: (e['http_status'] as num?)?.toInt() ?? res?.statusCode ?? 0,
        requestId: e['request_id']?.toString(),
        traceId: e['trace_id']?.toString(),
        retryable: e['retryable'] == true,
        fields: rawFields is Map
            ? rawFields.map((k, v) => MapEntry(k.toString(), v.toString()))
            : const {},
      );
    }
    return ApiError(
      code: 'INTERNAL',
      messageKey: 'errors.internal',
      status: res?.statusCode ?? 0,
      retryable: true,
    );
  }

  factory ApiError.fromDio(DioException e) {
    switch (e.type) {
      case DioExceptionType.connectionError:
      case DioExceptionType.connectionTimeout:
      case DioExceptionType.receiveTimeout:
      case DioExceptionType.sendTimeout:
        return ApiError.offline();
      case DioExceptionType.cancel:
        return ApiError(code: 'CANCELLED', messageKey: 'errors.cancelled', status: 0);
      default:
        return ApiError.fromResponse(e.response);
    }
  }

  @override
  String toString() => 'ApiError($code, status: $status, request: $requestId)';
}
