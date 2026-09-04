/// Zbritja e një kuponi, e llogaritur nga serveri. Klienti nuk shpik as përqindje as shumë.
class CouponApplied {
  const CouponApplied({required this.code, required this.kind, required this.discountMinor});

  final String code;

  /// percent | fixed — vetëm për shfaqje; shuma vjen gati.
  final String kind;
  final int discountMinor;

  factory CouponApplied.fromJson(Map<String, dynamic> j) => CouponApplied(
    code: (j['code'] ?? '').toString(),
    kind: (j['kind'] ?? 'fixed').toString(),
    discountMinor: (j['discount_minor'] as num?)?.toInt() ?? 0,
  );
}
