import 'package:flutter_test/flutter_test.dart';
import 'package:krejt_customer/screens/ride/review.dart';

// Etiketat e vlerësimit janë kontratë me serverin (reviews.CustomerTags): një emër i panjohur
// e refuzon gjithë vlerësimin me 422. Doli nga prova e udhëtimit të plotë — aplikacioni ofronte
// pesë etiketa të shpikura dhe asnjë vlerësim me etiketë nuk kalonte.
void main() {
  test('etiketat e vlerësimit janë vetëm ato që serveri i pranon për klientin', () {
    const server = {
      'clean_car',
      'friendly',
      'safe_driving',
      'great_route',
      'late',
      'rude',
      'unsafe_driving',
      'wrong_route',
      'dirty_car',
    };
    for (final tag in reviewTags) {
      expect(server, contains(tag), reason: '"$tag" nuk ekziston te serveri');
    }
  });

  test('etiketat nuk kalojnë kufirin e serverit (5)', () {
    expect(reviewTags.length, lessThanOrEqualTo(5));
  });
}
