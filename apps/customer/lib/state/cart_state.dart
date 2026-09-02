import 'package:flutter/foundation.dart';
import 'package:krejt_api/krejt_api.dart';

/// Shporta rri vetëm në pajisje dhe mban zgjedhje, jo çmime: sasi, produkte dhe opsione.
/// Sa kushton e thotë serveri në çdo hap, që çmimi në ekran të jetë ai që paguhet (§19).
class CartState extends ChangeNotifier {
  Merchant? merchant;
  List<CartLine> lines = const [];

  bool get isEmpty => lines.isEmpty;

  int get itemCount => lines.fold(0, (n, l) => n + l.quantity);

  bool belongsTo(String merchantId) => merchant?.id == merchantId;

  /// Një shportë mban një merchant të vetëm: një porosi shkon në një kuzhinë.
  void startAt(Merchant m) {
    if (merchant?.id == m.id) return;
    merchant = m;
    lines = const [];
    notifyListeners();
  }

  void add(CartLine line) {
    final updated = <CartLine>[];
    var merged = false;
    for (final existing in lines) {
      if (!merged && existing.sameAs(line)) {
        updated.add(existing.copyWith(quantity: existing.quantity + line.quantity));
        merged = true;
      } else {
        updated.add(existing);
      }
    }
    if (!merged) updated.add(line);
    lines = updated;
    notifyListeners();
  }

  void setQuantity(int index, int quantity) {
    if (index < 0 || index >= lines.length) return;
    final updated = [...lines];
    if (quantity <= 0) {
      updated.removeAt(index);
    } else {
      updated[index] = updated[index].copyWith(quantity: quantity);
    }
    lines = updated;
    if (lines.isEmpty) merchant = null;
    notifyListeners();
  }

  void clear() {
    lines = const [];
    merchant = null;
    notifyListeners();
  }
}
