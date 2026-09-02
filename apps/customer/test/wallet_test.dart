import 'package:flutter_test/flutter_test.dart';
import 'package:krejt_api/krejt_api.dart';
import 'package:krejt_customer/screens/wallet.dart';

void main() {
  final limits = WalletLimits(minTopupMinor: 100, maxTopupMinor: 50000, dailyTopupMinor: 100000);

  group('mbushja e wallet-it', () {
    test('shuma e vlefshme kalon', () {
      expect(checkTopUp(1000, limits), isNull);
      expect(checkTopUp(100, limits), isNull);
      expect(checkTopUp(50000, limits), isNull);
    });

    test('shuma bosh ose zero refuzohet', () {
      expect(checkTopUp(null, limits), TopUpProblem.empty);
      expect(checkTopUp(0, limits), TopUpProblem.empty);
      expect(checkTopUp(-500, limits), TopUpProblem.empty);
    });

    test('kufijtë e serverit respektohen në të dyja skajet', () {
      expect(checkTopUp(50, limits), TopUpProblem.tooSmall);
      expect(checkTopUp(50050, limits), TopUpProblem.tooLarge);
    });

    test('shuma që nuk është shumëfish i 0,50 € refuzohet', () {
      expect(checkTopUp(1099, limits), TopUpProblem.notMultiple);
      expect(checkTopUp(1050, limits), isNull);
    });
  });

  group('leximi i shumës së shkruar', () {
    test('presja dhe pika trajtohen njësoj', () {
      expect(parseAmountMinor('12,50'), 1250);
      expect(parseAmountMinor('12.50'), 1250);
      expect(parseAmountMinor(' 7 '), 700);
    });

    test('teksti që nuk është numër kthen null', () {
      expect(parseAmountMinor(''), isNull);
      expect(parseAmountMinor('dhjetë'), isNull);
    });

    test('rrumbullakimi nuk humb cent', () {
      expect(parseAmountMinor('0.1'), 10);
      expect(parseAmountMinor('19.99'), 1999);
    });
  });
}
