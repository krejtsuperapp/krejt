import 'package:flutter/material.dart';

import '../tokens.dart';

/// Paraqitja e parasë. Vlera vjen gjithmonë si numër i plotë në cent (§5) dhe formatohet këtu,
/// që asnjë ekran të mos shpikë formatin e vet.
String formatMinor(int minor, {String currency = 'EUR', String locale = 'sq', bool sign = false}) {
  final neg = minor < 0;
  final v = neg ? -minor : minor;
  final whole = (v ~/ 100).toString();
  final cents = (v % 100).toString().padLeft(2, '0');
  final symbol = currency == 'EUR' ? '€' : currency;
  final body = locale == 'en' ? '$symbol$whole.$cents' : '$whole,$cents $symbol';
  if (neg) return '-$body';
  return sign ? '+$body' : body;
}

/// Shuma e madhe që lexohet me një shikim: çmimi i udhëtimit, bilanci i wallet-it, totali i porosisë.
class KMoney extends StatelessWidget {
  const KMoney(
    this.minor, {
    super.key,
    this.currency = 'EUR',
    this.locale = 'sq',
    this.size = 28,
    this.color,
    this.strikethrough = false,
    this.signed = false,
  });

  final int minor;
  final String currency;
  final String locale;
  final double size;
  final Color? color;
  final bool strikethrough;
  final bool signed;

  @override
  Widget build(BuildContext context) {
    final c = color ?? (signed && minor >= 0 ? K.ok : K.text);
    return Text(
      formatMinor(minor, currency: currency, locale: locale, sign: signed),
      style: TextStyle(
        fontSize: size,
        height: 1.1,
        fontWeight: FontWeight.w700,
        letterSpacing: -0.5,
        color: c,
        fontFeatures: const [FontFeature.tabularFigures()],
        decoration: strikethrough ? TextDecoration.lineThrough : null,
        decorationColor: K.muted,
      ),
    );
  }
}

/// Rresht i një zërti të faturës: etiketa majtas, shuma djathtas me shifra që rreshtohen.
class KMoneyRow extends StatelessWidget {
  const KMoneyRow(
    this.label,
    this.minor, {
    super.key,
    this.currency = 'EUR',
    this.locale = 'sq',
    this.total = false,
    this.hint,
  });

  final String label;
  final int minor;
  final String currency;
  final String locale;
  final bool total;
  final String? hint;

  @override
  Widget build(BuildContext context) {
    final labelStyle = TextStyle(
      fontSize: total ? 15 : 14,
      fontWeight: total ? FontWeight.w600 : FontWeight.w500,
      color: total ? K.text : K.textDim,
    );
    final valueStyle = TextStyle(
      fontSize: total ? 18 : 14,
      fontWeight: total ? FontWeight.w700 : FontWeight.w600,
      color: total ? K.text : K.textDim,
      fontFeatures: const [FontFeature.tabularFigures()],
    );
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: K.s1 + 2),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(label, style: labelStyle),
                if (hint != null)
                  Padding(
                    padding: const EdgeInsets.only(top: 2),
                    child: Text(hint!, style: const TextStyle(fontSize: 12, color: K.muted)),
                  ),
              ],
            ),
          ),
          const SizedBox(width: K.s3),
          Text(
            formatMinor(minor, currency: currency, locale: locale),
            style: valueStyle,
          ),
        ],
      ),
    );
  }
}

/// Ndarës i hollë mes zërave të faturës dhe totalit.
class KMoneyDivider extends StatelessWidget {
  const KMoneyDivider({super.key});

  @override
  Widget build(BuildContext context) => const Padding(
    padding: EdgeInsets.symmetric(vertical: K.s2),
    child: Divider(height: 1, thickness: 1, color: K.line),
  );
}
