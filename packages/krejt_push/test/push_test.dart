import 'package:flutter_test/flutter_test.dart';
import 'package:krejt_push/krejt_push.dart';

void main() {
  group('PushConfig', () {
    test('ndizet vetëm me të katër vlerat', () {
      final c = PushConfig.fromEnv(apiKey: 'k', appId: 'a', senderId: 's', projectId: 'p');
      expect(c.enabled, isTrue);
      expect(c.options.messagingSenderId, 's');
    });

    test('gjysma e konfigurimit nuk ndez asgjë', () {
      final c = PushConfig.fromEnv(apiKey: 'k', appId: 'a', senderId: '', projectId: 'p');
      expect(c.enabled, isFalse);
    });

    test('pa asnjë vlerë — e fikur, pa gabim', () {
      final c = PushConfig.fromEnv(apiKey: '', appId: '', senderId: '', projectId: '');
      expect(c.enabled, isFalse);
    });
  });
}
