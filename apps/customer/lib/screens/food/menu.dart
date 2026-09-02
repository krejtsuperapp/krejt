import 'package:flutter/material.dart';
import 'package:krejt_api/krejt_api.dart';
import 'package:krejt_design/krejt_design.dart';
import 'package:krejt_l10n/krejt_l10n.dart';
import 'package:provider/provider.dart';

import '../../state/app_state.dart';
import '../../state/cart_state.dart';
import 'cart.dart';

/// Menuja e një vendi. Produktet pa disponueshmëri shfaqen të shuara dhe nuk shtohen dot,
/// që askush të mos porosisë diçka që kuzhina do ta anulojë (§19).
class MenuScreen extends StatefulWidget {
  const MenuScreen({super.key, required this.merchant});

  final Merchant merchant;

  @override
  State<MenuScreen> createState() => _MenuScreenState();
}

class _MenuScreenState extends State<MenuScreen> {
  Menu? _menu;
  bool _loading = true;
  ApiError? _error;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _load());
  }

  Future<void> _load() async {
    if (mounted) setState(() => _loading = true);
    try {
      final menu = await context.read<AppState>().api.merchantMenu(widget.merchant.id);
      if (!mounted) return;
      setState(() {
        _menu = menu;
        _error = null;
        _loading = false;
      });
    } on ApiError catch (e) {
      if (!mounted) return;
      setState(() {
        _error = e;
        _loading = false;
      });
    }
  }

  /// Shtimi nga një vend tjetër e zbraz shportën, por vetëm pasi përdoruesi e pranon.
  Future<bool> _ensureCartMerchant() async {
    final cart = context.read<CartState>();
    if (cart.isEmpty || cart.belongsTo(widget.merchant.id)) {
      cart.startAt(widget.merchant);
      return true;
    }
    final ok = await confirmKSheet(
      context: context,
      title: context.t('cart.other_merchant'),
      message: context.t('cart.other_merchant.body'),
      confirmLabel: context.t('common.continue'),
      cancelLabel: context.t('common.cancel'),
      destructive: true,
    );
    if (!ok || !mounted) return false;
    cart.clear();
    cart.startAt(widget.merchant);
    return true;
  }

  Future<void> _openProduct(Product product) async {
    final line = await showKSheet<CartLine>(
      context: context,
      title: product.name,
      subtitle: product.description,
      scrollable: true,
      child: _ProductSheet(product: product),
    );
    if (line == null || !mounted) return;
    if (!await _ensureCartMerchant()) return;
    if (!mounted) return;
    context.read<CartState>().add(line);
  }

  @override
  Widget build(BuildContext context) {
    final cart = context.watch<CartState>();
    final showCart = !cart.isEmpty && cart.belongsTo(widget.merchant.id);

    return Scaffold(
      backgroundColor: K.bg,
      appBar: AppBar(title: Text(widget.merchant.name)),
      floatingActionButton: showCart
          ? FloatingActionButton.extended(
              backgroundColor: K.brand500,
              foregroundColor: K.onBrand,
              icon: const Icon(Icons.shopping_bag_outlined),
              label: Text('${context.t('cart.title')} · ${cart.itemCount}'),
              onPressed: () =>
                  Navigator.of(context)
                      .push(MaterialPageRoute<void>(builder: (_) => const CartScreen())),
            )
          : null,
      body: SafeArea(child: _body(context)),
    );
  }

  Widget _body(BuildContext context) {
    final locale = context.watch<AppState>().locale;
    if (_loading) {
      return const Padding(padding: EdgeInsets.all(K.s5), child: KSkeleton(height: 72));
    }
    final menu = _menu;
    if (menu == null) {
      return Padding(
        padding: const EdgeInsets.all(K.s5),
        child: KError(
          message: context.tError(_error?.messageKey ?? 'errors.internal'),
          retryLabel: context.t('common.retry'),
          onRetry: _load,
        ),
      );
    }

    final uncategorised = menu.products.where((p) => p.categoryId == null).toList();

    return ListView(
      padding: const EdgeInsets.fromLTRB(K.s5, K.s4, K.s5, 96),
      children: [
        KCard(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  KBadge(
                    context.t(widget.merchant.canOrder ? 'food.open' : 'food.closed'),
                    tone: widget.merchant.canOrder ? KTone.ok : KTone.neutral,
                  ),
                  const SizedBox(width: K.s2),
                  Expanded(
                    child: Text(
                      '${widget.merchant.addressLine1}, ${widget.merchant.city}',
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: const TextStyle(fontSize: 12, color: K.muted),
                    ),
                  ),
                ],
              ),
              const SizedBox(height: K.s2),
              Text(
                [
                  context.t('food.prep', {'min': '${widget.merchant.prepTimeMin}'}),
                  context.t('food.min_order', {
                    'amount': formatMinor(widget.merchant.minOrderMinor, locale: locale),
                  }),
                ].join(' · '),
                style: const TextStyle(fontSize: 12, color: K.textDim),
              ),
            ],
          ),
        ),
        const SizedBox(height: K.s5),
        for (final category in menu.categories) ...[
          KSectionHeader(category.name),
          const SizedBox(height: K.s3),
          for (final product in menu.inCategory(category.id))
            _ProductRow(product: product, locale: locale, onTap: () => _openProduct(product)),
          const SizedBox(height: K.s4),
        ],
        for (final product in uncategorised)
          _ProductRow(product: product, locale: locale, onTap: () => _openProduct(product)),
      ],
    );
  }
}

class _ProductRow extends StatelessWidget {
  const _ProductRow({required this.product, required this.locale, required this.onTap});

  final Product product;
  final String locale;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final available = product.available;
    return Padding(
      padding: const EdgeInsets.only(bottom: K.s2),
      child: KCard(
        onTap: available ? onTap : null,
        child: Row(
          children: [
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    product.name,
                    style: TextStyle(
                      fontSize: 15,
                      fontWeight: FontWeight.w600,
                      color: available ? K.text : K.muted,
                    ),
                  ),
                  if (product.description != null)
                    Padding(
                      padding: const EdgeInsets.only(top: 2),
                      child: Text(
                        product.description!,
                        maxLines: 2,
                        overflow: TextOverflow.ellipsis,
                        style: const TextStyle(fontSize: 12, color: K.muted, height: 1.35),
                      ),
                    ),
                  if (!available)
                    Padding(
                      padding: const EdgeInsets.only(top: K.s2),
                      child: KBadge(context.t('menu.unavailable'), tone: KTone.neutral),
                    ),
                ],
              ),
            ),
            const SizedBox(width: K.s3),
            KMoney(
              product.priceMinor,
              currency: product.currency,
              locale: locale,
              size: 16,
              color: available ? K.text : K.muted,
            ),
          ],
        ),
      ),
    );
  }
}

/// Zgjedhja e modifikuesve. Grupet e detyrueshme e mbajnë butonin të fikur derisa të plotësohen,
/// që porosia të mos shkojë e paplotë në kuzhinë.
class _ProductSheet extends StatefulWidget {
  const _ProductSheet({required this.product});

  final Product product;

  @override
  State<_ProductSheet> createState() => _ProductSheetState();
}

class _ProductSheetState extends State<_ProductSheet> {
  final _selected = <String, Set<String>>{};
  int _quantity = 1;

  bool get _complete {
    for (final group in widget.product.modifiers) {
      final picked = _selected[group.id]?.length ?? 0;
      if (picked < group.minSelect) return false;
    }
    return true;
  }

  void _toggle(ModifierGroup group, ModifierOption option) {
    final picked = _selected.putIfAbsent(group.id, () => <String>{});
    setState(() {
      if (group.single) {
        picked
          ..clear()
          ..add(option.id);
        return;
      }
      if (picked.contains(option.id)) {
        picked.remove(option.id);
      } else if (picked.length < group.maxSelect) {
        picked.add(option.id);
      }
    });
  }

  @override
  Widget build(BuildContext context) {
    final locale = context.watch<AppState>().locale;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        for (final group in widget.product.modifiers) ...[
          Row(
            children: [
              Expanded(
                child: Text(
                  group.name,
                  style: const TextStyle(fontSize: 15, fontWeight: FontWeight.w700, color: K.text),
                ),
              ),
              KBadge(
                group.required
                    ? context.t('menu.required', {'n': '${group.minSelect}'})
                    : context.t('menu.choose_upto', {'n': '${group.maxSelect}'}),
                tone: group.required ? KTone.warn : KTone.neutral,
              ),
            ],
          ),
          const SizedBox(height: K.s2),
          for (final option in group.options)
            SizedBox(
              height: K.minTap,
              child: Row(
                children: [
                  Expanded(
                    child: Text(
                      option.name,
                      style: TextStyle(fontSize: 14, color: option.available ? K.textDim : K.muted),
                    ),
                  ),
                  if (option.priceDeltaMinor != 0)
                    Padding(
                      padding: const EdgeInsets.only(right: K.s2),
                      child: KMoney(
                        option.priceDeltaMinor,
                        locale: locale,
                        size: 13,
                        signed: true,
                        color: K.muted,
                      ),
                    ),
                  Checkbox(
                    value: _selected[group.id]?.contains(option.id) ?? false,
                    onChanged: option.available ? (_) => _toggle(group, option) : null,
                  ),
                ],
              ),
            ),
          const SizedBox(height: K.s4),
        ],
        Row(
          children: [
            IconButton.filledTonal(
              onPressed: _quantity > 1 ? () => setState(() => _quantity--) : null,
              icon: const Icon(Icons.remove),
            ),
            Padding(
              padding: const EdgeInsets.symmetric(horizontal: K.s4),
              child: Text(
                '$_quantity',
                style: const TextStyle(fontSize: 18, fontWeight: FontWeight.w700, color: K.text),
              ),
            ),
            IconButton.filledTonal(
              onPressed: _quantity < 50 ? () => setState(() => _quantity++) : null,
              icon: const Icon(Icons.add),
            ),
            const Spacer(),
            KMoney(
              widget.product.priceMinor * _quantity,
              currency: widget.product.currency,
              locale: locale,
              size: 20,
            ),
          ],
        ),
        const SizedBox(height: K.s4),
        KButton(
          label: context.t('menu.add'),
          onPressed: _complete
              ? () => Navigator.of(context).pop(
                  CartLine(
                    product: widget.product,
                    optionIds: _selected.values.expand((s) => s).toList(),
                    quantity: _quantity,
                  ),
                )
              : null,
        ),
      ],
    );
  }
}
