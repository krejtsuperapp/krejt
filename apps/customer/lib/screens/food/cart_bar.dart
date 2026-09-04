import 'package:flutter/material.dart';
import 'package:krejt_design/krejt_design.dart';
import 'package:krejt_l10n/krejt_l10n.dart';
import 'package:provider/provider.dart';

import '../../state/cart_state.dart';
import 'cart.dart';

/// Shiriti i shportës mbi listën e vendeve.
///
/// Pa të, shporta shihej vetëm brenda menusë: shtoje një pjatë, kthehu te lista, dhe ajo zhdukej
/// nga sytë — përdoruesi duhej të mbante mend vetë te cili lokal e kishte lënë. Ky është një nga
/// hapat ku porositë humbasin te aplikacionet e ushqimit, dhe kushton një rresht.
///
/// Nuk shfaqet kur shporta është bosh, dhe mban emrin e lokalit sepse një shportë i takon një
/// kuzhine të vetme: pa emrin, "3 artikuj" nuk të thotë ku.
class CartBar extends StatelessWidget {
  const CartBar({super.key});

  @override
  Widget build(BuildContext context) {
    final cart = context.watch<CartState>();
    final merchant = cart.merchant;
    if (cart.isEmpty || merchant == null) return const SizedBox.shrink();

    return SafeArea(
      top: false,
      child: Padding(
        padding: const EdgeInsets.fromLTRB(K.s5, 0, K.s5, K.s3),
        child: Material(
          color: K.brand500,
          borderRadius: BorderRadius.circular(K.rMd),
          child: InkWell(
            borderRadius: BorderRadius.circular(K.rMd),
            onTap: () =>
                Navigator.of(context)
                    .push(MaterialPageRoute<void>(builder: (_) => const CartScreen())),
            child: Padding(
              padding: const EdgeInsets.symmetric(horizontal: K.s4, vertical: K.s4),
              child: Row(
                children: [
                  Container(
                    padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
                    decoration: BoxDecoration(
                      color: Colors.white.withValues(alpha: 0.22),
                      borderRadius: BorderRadius.circular(K.rFull),
                    ),
                    child: Text(
                      '${cart.itemCount}',
                      style: const TextStyle(
                        fontSize: 13,
                        fontWeight: FontWeight.w800,
                        color: Colors.white,
                      ),
                    ),
                  ),
                  const SizedBox(width: K.s3),
                  Expanded(
                    child: Text(
                      merchant.name,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: const TextStyle(
                        fontSize: 15,
                        fontWeight: FontWeight.w700,
                        color: Colors.white,
                      ),
                    ),
                  ),
                  const SizedBox(width: K.s3),
                  Text(
                    context.t('food.cart.open'),
                    style: const TextStyle(
                      fontSize: 14,
                      fontWeight: FontWeight.w700,
                      color: Colors.white,
                    ),
                  ),
                  const Icon(Icons.chevron_right, size: 20, color: Colors.white),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }
}
