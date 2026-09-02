/// Paraja në KREJT është gjithmonë numër i plotë në cent (§5, §23) — asnjë `double` në asnjë rrjedhë.
library;

extension MoneyMinor on int {
  /// "12,40 €" (sq/de) ose "€12.40" (en). Negativja shfaqet me minus para simbolit.
  String money({String currency = 'EUR', String locale = 'sq'}) {
    final neg = this < 0;
    final v = neg ? -this : this;
    final whole = v ~/ 100;
    final cents = (v % 100).toString().padLeft(2, '0');
    final symbol = currency == 'EUR' ? '€' : currency;
    final s = locale == 'en' ? '$symbol$whole.$cents' : '$whole,$cents $symbol';
    return neg ? '-$s' : s;
  }
}

/// Formatim i distancës dhe kohës për ekranet e udhëtimit dhe dorëzimit.
String formatDistance(int meters, {String locale = 'sq'}) {
  if (meters < 950) return '$meters m';
  final km = (meters / 100).round() / 10;
  final text = km.toStringAsFixed(km >= 10 ? 0 : 1);
  return locale == 'en' ? '${text.replaceAll(',', '.')} km' : '${text.replaceAll('.', ',')} km';
}

String formatDuration(int seconds) {
  if (seconds < 60) return '< 1 min';
  final m = (seconds / 60).round();
  if (m < 60) return '$m min';
  final h = m ~/ 60;
  final rem = m % 60;
  return rem == 0 ? '$h h' : '$h h $rem min';
}
