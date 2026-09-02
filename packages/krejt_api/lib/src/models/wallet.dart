/// Wallet-i i mbyllur dhe pagesat me kartë (§23, §29). Bilanci llogaritet nga ledger-i, kurrë nga klienti.
library;

class WalletLimits {
  WalletLimits({
    required this.minTopupMinor,
    required this.maxTopupMinor,
    required this.dailyTopupMinor,
  });

  final int minTopupMinor;
  final int maxTopupMinor;
  final int dailyTopupMinor;

  factory WalletLimits.fromJson(Map<String, dynamic> j) => WalletLimits(
    minTopupMinor: (j['min_topup_minor'] as num?)?.toInt() ?? 100,
    maxTopupMinor: (j['max_topup_minor'] as num?)?.toInt() ?? 50000,
    dailyTopupMinor: (j['daily_topup_minor'] as num?)?.toInt() ?? 100000,
  );
}

class WalletOverview {
  WalletOverview({
    required this.balanceMinor,
    required this.currency,
    required this.closedLoop,
    required this.limits,
  });

  final int balanceMinor;
  final String currency;
  final bool closedLoop;
  final WalletLimits limits;

  factory WalletOverview.fromJson(Map<String, dynamic> j) => WalletOverview(
    balanceMinor: (j['balance_minor'] as num?)?.toInt() ?? 0,
    currency: (j['currency'] ?? 'EUR').toString(),
    closedLoop: j['closed_loop'] != false,
    limits: WalletLimits.fromJson(Map<String, dynamic>.from((j['limits'] as Map?) ?? const {})),
  );
}

class WalletTransaction {
  WalletTransaction({
    required this.id,
    required this.kind,
    required this.reference,
    required this.amountMinor,
    required this.currency,
    required this.createdAt,
  });

  final String id;
  final String kind;
  final String reference;

  /// Pozitive = hyrje, negative = dalje.
  final int amountMinor;
  final String currency;
  final DateTime createdAt;

  bool get isCredit => amountMinor >= 0;

  factory WalletTransaction.fromJson(Map<String, dynamic> j) => WalletTransaction(
    id: j['id'].toString(),
    kind: (j['kind'] ?? 'other').toString(),
    reference: (j['reference'] ?? '').toString(),
    amountMinor: (j['amount_minor'] as num?)?.toInt() ?? 0,
    currency: (j['currency'] ?? 'EUR').toString(),
    createdAt: DateTime.tryParse(j['created_at']?.toString() ?? '') ?? DateTime.now(),
  );
}

enum PaymentStatus { created, requiresAction, processing, succeeded, failed, canceled }

class PaymentIntent {
  PaymentIntent({
    required this.id,
    required this.purpose,
    required this.amountMinor,
    required this.currency,
    required this.provider,
    required this.status,
    this.failureCode,
    this.clientSecret,
    required this.createdAt,
    this.succeededAt,
  });

  final String id;
  final String purpose;
  final int amountMinor;
  final String currency;
  final String provider;
  final PaymentStatus status;
  final String? failureCode;
  final String? clientSecret;
  final DateTime createdAt;
  final DateTime? succeededAt;

  bool get isSettled => status == PaymentStatus.succeeded;
  bool get isPending =>
      status == PaymentStatus.created ||
      status == PaymentStatus.requiresAction ||
      status == PaymentStatus.processing;

  factory PaymentIntent.fromJson(Map<String, dynamic> j) {
    final raw = (j['status'] ?? 'created').toString();
    final status = PaymentStatus.values.firstWhere(
      (s) => s.name == raw || (s == PaymentStatus.requiresAction && raw == 'requires_action'),
      orElse: () => PaymentStatus.created,
    );
    return PaymentIntent(
      id: j['id'].toString(),
      purpose: (j['purpose'] ?? '').toString(),
      amountMinor: (j['amount_minor'] as num?)?.toInt() ?? 0,
      currency: (j['currency'] ?? 'EUR').toString(),
      provider: (j['provider'] ?? '').toString(),
      status: status,
      failureCode: j['failure_code']?.toString(),
      clientSecret: j['client_secret']?.toString(),
      createdAt: DateTime.tryParse(j['created_at']?.toString() ?? '') ?? DateTime.now(),
      succeededAt: DateTime.tryParse(j['succeeded_at']?.toString() ?? ''),
    );
  }
}

class AppNotification {
  AppNotification({
    required this.id,
    required this.category,
    required this.titleKey,
    required this.bodyKey,
    required this.params,
    this.deepLink,
    this.readAt,
    required this.createdAt,
  });

  final String id;
  final String category;
  final String titleKey;
  final String bodyKey;
  final Map<String, String> params;
  final String? deepLink;
  final DateTime? readAt;
  final DateTime createdAt;

  bool get unread => readAt == null;

  factory AppNotification.fromJson(Map<String, dynamic> j) {
    final p = <String, String>{};
    final raw = j['params'];
    if (raw is Map) {
      raw.forEach((k, v) => p[k.toString()] = v.toString());
    }
    return AppNotification(
      id: j['id'].toString(),
      category: (j['category'] ?? 'general').toString(),
      titleKey: (j['title_key'] ?? '').toString(),
      bodyKey: (j['body_key'] ?? '').toString(),
      params: p,
      deepLink: j['deep_link']?.toString(),
      readAt: DateTime.tryParse(j['read_at']?.toString() ?? ''),
      createdAt: DateTime.tryParse(j['created_at']?.toString() ?? '') ?? DateTime.now(),
    );
  }
}
