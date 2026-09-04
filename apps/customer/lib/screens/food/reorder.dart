import 'package:flutter/material.dart';
import 'package:krejt_api/krejt_api.dart';
import 'package:krejt_design/krejt_design.dart';
import 'package:krejt_l10n/krejt_l10n.dart';
import 'package:provider/provider.dart';

import '../../state/app_state.dart';
import '../../state/cart_state.dart';
import 'cart.dart';

/// Rezultati i rindërtimit të një shporte nga një porosi e vjetër.
///
/// Çmimet dhe disponueshmëria vijnë gjithmonë nga menuja e sotme, kurrë nga porosia e ruajtur:
/// një pjatë mund të jetë shtrenjtuar, hequr nga menuja ose mbaruar. Prandaj rezultati e thotë
/// hapur se çfarë nuk u shtua — më mirë se një shportë që duket e plotë dhe nuk është.
class ReorderResult {
  const ReorderResult({required this.lines, required this.missing});

  final List<CartLine> lines;

  /// Emrat e artikujve që nuk ekzistojnë më ose nuk shiten sot.
  final List<String> missing;

  bool get isEmpty => lines.isEmpty;
}

/// Rindërton shportën nga porosia duke u nisur nga produkti, jo nga emri: emrat ndryshojnë, dhe
/// dy pjata mund të quhen njësoj. Opsionet mbahen vetëm ato që ekzistojnë ende te produkti.
ReorderResult rebuildCart(Order order, Menu menu) {
  final byId = {for (final p in menu.products) p.id: p};
  final lines = <CartLine>[];
  final missing = <String>[];

  for (final item in order.items) {
    final product = byId[item.productId];
    if (product == null || !product.available) {
      missing.add(item.name);
      continue;
    }
    final valid = {
      for (final g in product.modifiers)
        for (final o in g.options) o.id,
    };
    lines.add(
      CartLine(
        product: product,
        optionIds: item.optionIds.where(valid.contains).toList(),
        quantity: item.quantity,
      ),
    );
  }
  return ReorderResult(lines: lines, missing: missing);
}

/// Merr menunë e sotme, rindërton shportën dhe hap shportën. Shporta e mëparshme zëvendësohet:
/// një shportë mban një lokal të vetëm, dhe përdoruesi sapo tha se do këtë.
Future<void> reorderOrder(BuildContext context, Order order) async {
  final messenger = ScaffoldMessenger.of(context);
  final l10n = KL10n.of(context);
  final state = context.read<AppState>();
  final cart = context.read<CartState>();

  showDialog<void>(
    context: context,
    barrierDismissible: false,
    builder: (_) => const Center(child: CircularProgressIndicator(color: K.brand400)),
  );

  try {
    final merchant = await state.api.merchantBySlug(order.merchantId);
    final menu = await state.api.merchantMenu(order.merchantId);
    final result = rebuildCart(order, menu);
    if (!context.mounted) return;
    Navigator.of(context).pop(); // mbyll pritjen

    if (result.isEmpty) {
      messenger.showSnackBar(SnackBar(content: Text(l10n.t('food.reorder.gone'))));
      return;
    }
    cart.startAt(merchant);
    for (final line in result.lines) {
      cart.add(line);
    }
    if (result.missing.isNotEmpty) {
      messenger.showSnackBar(
        SnackBar(
          content: Text(l10n.t('food.reorder.missing', {'items': result.missing.join(', ')})),
        ),
      );
    }
    await Navigator.of(context).push(MaterialPageRoute<void>(builder: (_) => const CartScreen()));
  } on ApiError catch (e) {
    if (!context.mounted) return;
    Navigator.of(context).pop();
    messenger.showSnackBar(SnackBar(content: Text(l10n.error(e.messageKey))));
  }
}
