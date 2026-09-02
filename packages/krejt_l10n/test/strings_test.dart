import 'package:flutter_test/flutter_test.dart';
import 'package:krejt_l10n/krejt_l10n.dart';
import 'package:krejt_l10n/src/strings.dart';

void main() {
  test('çdo çelës i shqipes ekziston edhe në anglisht dhe gjermanisht', () {
    final sq = kStrings['sq']!;
    for (final lang in ['en', 'de']) {
      final missing = sq.keys.where((k) => !kStrings[lang]!.containsKey(k)).toList()..sort();
      expect(missing, isEmpty, reason: 'mungojnë në $lang: ${missing.join(', ')}');
    }
  });

  test('asnjë gjuhë nuk ka çelësa që shqipja s\'i ka', () {
    final sq = kStrings['sq']!;
    for (final lang in ['en', 'de']) {
      final extra = kStrings[lang]!.keys.where((k) => !sq.containsKey(k)).toList()..sort();
      expect(extra, isEmpty, reason: 'tepër në $lang: ${extra.join(', ')}');
    }
  });

  test('vendmbajtësit {…} përputhen mes gjuhëve', () {
    final re = RegExp(r'\{(\w+)\}');
    final sq = kStrings['sq']!;
    for (final entry in sq.entries) {
      final want = re.allMatches(entry.value).map((m) => m[1]!).toSet();
      for (final lang in ['en', 'de']) {
        final got = re.allMatches(kStrings[lang]![entry.key]!).map((m) => m[1]!).toSet();
        expect(got, want, reason: '${entry.key} në $lang');
      }
    }
  });

  test('gjuhët e mbështetura janë sq, en, de dhe shqipja është e para', () {
    expect(kSupportedLocales.map((l) => l.languageCode).toList(), ['sq', 'en', 'de']);
    expect(kDefaultLocale.languageCode, 'sq');
  });

  test('çelësi që mungon kthen vetveten, jo rrëzim', () {
    final l10n = KL10n(kDefaultLocale);
    expect(l10n.t('nuk.ekziston'), 'nuk.ekziston');
  });

  test('parametrat zëvendësohen në tekst', () {
    final l10n = KL10n(kDefaultLocale);
    expect(l10n.t('home.greeting', {'name': 'Arta'}), contains('Arta'));
  });

  test('gabimi i panjohur bie te mesazhi i përgjithshëm', () {
    final l10n = KL10n(kDefaultLocale);
    expect(l10n.error('errors.diçka.e.re'), kStrings['sq']!['errors.internal']);
  });
}
