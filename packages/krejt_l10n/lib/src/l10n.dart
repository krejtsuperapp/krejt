import 'package:flutter/widgets.dart';

import 'strings.dart';

/// Qasja te përkthimet. `context.t('home.title')` kthen tekstin e gjuhës aktive;
/// nëse çelësi mungon, bie te shqipja dhe si zgjidhja e fundit kthen vetë çelësin,
/// që mungesa të duket në ekran gjatë testimit e jo të shkaktojë rrëzim.
class KL10n {
  KL10n(this.locale);

  final Locale locale;

  String get code => locale.languageCode;

  static KL10n of(BuildContext context) =>
      Localizations.of<KL10n>(context, KL10n) ?? KL10n(const Locale('sq'));

  static const LocalizationsDelegate<KL10n> delegate = _KL10nDelegate();

  String t(String key, [Map<String, String>? params]) {
    final table = kStrings[code] ?? kStrings['sq']!;
    var value = table[key] ?? kStrings['sq']![key] ?? key;
    if (params != null) {
      params.forEach((k, v) => value = value.replaceAll('{$k}', v));
    }
    return value;
  }

  /// Përkthimi i një gabimi të API-së nga `message_key`; nëse serveri dërgon çelës të panjohur,
  /// përdoruesi sheh mesazhin e përgjithshëm, kurrë tekstin teknik.
  String error(String messageKey) {
    final table = kStrings[code] ?? kStrings['sq']!;
    return table[messageKey] ?? kStrings['sq']![messageKey] ?? t('errors.internal');
  }

  /// Emri i gjuhës në gjuhën e vet, për listën e zgjedhjes.
  static String languageName(String code) {
    switch (code) {
      case 'en':
        return 'English';
      case 'de':
        return 'Deutsch';
      default:
        return 'Shqip';
    }
  }
}

class _KL10nDelegate extends LocalizationsDelegate<KL10n> {
  const _KL10nDelegate();

  @override
  bool isSupported(Locale locale) => kStrings.containsKey(locale.languageCode);

  @override
  Future<KL10n> load(Locale locale) async => KL10n(locale);

  @override
  bool shouldReload(_KL10nDelegate old) => false;
}

extension KL10nContext on BuildContext {
  String t(String key, [Map<String, String>? params]) => KL10n.of(this).t(key, params);
  String tError(String messageKey) => KL10n.of(this).error(messageKey);
  String get languageCode => KL10n.of(this).code;
}
