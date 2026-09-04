import 'dart:async';

import 'package:flutter/material.dart';
import 'package:krejt_api/krejt_api.dart';
import 'package:krejt_design/krejt_design.dart';
import 'package:krejt_l10n/krejt_l10n.dart';
import 'package:provider/provider.dart';

import '../../services/location.dart';
import '../../state/app_state.dart';
import '../active_banner.dart';
import 'cart_bar.dart';
import 'menu.dart';

const merchantTypes = ['restaurant', 'store', 'grocery', 'pharmacy'];

String merchantTypeKey(String type) =>
    'food.type.${merchantTypes.contains(type) ? type : 'restaurant'}';

/// Ushqimi: restorantet e hapura tani afër teje. Ushqimoret, farmacitë dhe dyqanet i takojnë
/// Marketit — një ekran që tregon të dyja do t'i shërbente keq secilës.
///
/// Filtri është kuzhina, jo tipi i tregtarit: kur të gjitha rezultatet janë restorante, "kuzhinë
/// shqiptare" apo "pica" është ajo që njeriu vërtet kërkon. Lista e kuzhinave nuk vjen nga një
/// listë e fiksuar në kod, por nga vetë vendet afër — kështu tregon çka ekziston, jo çka shpresojmë.
///
/// Vendet e mbyllura shfaqen poshtë dhe të shuara, jo të fshehura — që përdoruesi ta dijë se
/// ekzistojnë dhe kur kthehen (§21).
class DiscoverScreen extends StatefulWidget {
  const DiscoverScreen({super.key});

  @override
  State<DiscoverScreen> createState() => _DiscoverScreenState();
}

class _DiscoverScreenState extends State<DiscoverScreen> {
  static const _location = LocationService();

  final _search = TextEditingController();

  List<Merchant> _items = const [];

  /// Kuzhinat e para nga ngarkimi pa filtër; pa këtë, zgjedhja e njërës do t'i zhdukte të tjerat.
  List<String> _cuisines = const [];
  LatLng? _at;
  String? _cuisine;
  bool _loading = true;
  ApiError? _error;
  LocationProblem? _locationProblem;
  Timer? _debounce;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _load());
  }

  @override
  void dispose() {
    _debounce?.cancel();
    _search.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    if (mounted) setState(() => _loading = true);
    var at = _at;
    if (at == null) {
      final position = await _location.current();
      if (!mounted) return;
      if (!position.isOk) {
        setState(() {
          _locationProblem = position.problem;
          _loading = false;
        });
        return;
      }
      at = position.point;
      _at = at;
    }
    try {
      final items = await context.read<AppState>().api.merchants(
        lat: at!.lat,
        lng: at.lng,
        type: 'restaurant',
        cuisine: _cuisine,
        query: _search.text.trim(),
      );
      if (!mounted) return;
      setState(() {
        _items = items;
        if (_cuisine == null && _search.text.trim().isEmpty) {
          _cuisines = {for (final m in items) ...m.cuisines}.toList()..sort();
        }
        _error = null;
        _locationProblem = null;
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

  /// Kërkimi pret gjysmë sekonde pas shkronjës së fundit, që të mos dërgojë një kërkesë për çdo prekje.
  void _onQueryChanged(String _) {
    _debounce?.cancel();
    _debounce = Timer(const Duration(milliseconds: 500), _load);
  }

  @override
  Widget build(BuildContext context) {
    final open = _items.where((m) => m.canOrder).toList();
    final closed = _items.where((m) => !m.canOrder).toList();

    return Scaffold(
      backgroundColor: K.bg,
      appBar: AppBar(title: Text(context.t('food.title'))),
      body: SafeArea(
        child: Column(
          children: [
            Padding(
              padding: const EdgeInsets.fromLTRB(K.s5, K.s2, K.s5, K.s3),
              child: KField(
                label: context.t('common.search'),
                controller: _search,
                hint: context.t('food.search'),
                onChanged: _onQueryChanged,
                onSubmitted: (_) => _load(),
              ),
            ),
            SizedBox(
              height: 44,
              child: ListView(
                scrollDirection: Axis.horizontal,
                padding: const EdgeInsets.symmetric(horizontal: K.s5),
                children: [
                  for (final cuisine in _cuisines)
                    Padding(
                      padding: const EdgeInsets.only(right: K.s2),
                      child: ChoiceChip(
                        label: Text(cuisine),
                        selected: _cuisine == cuisine,
                        onSelected: (on) {
                          setState(() => _cuisine = on ? cuisine : null);
                          _load();
                        },
                      ),
                    ),
                ],
              ),
            ),
            Expanded(child: _body(context, open, closed)),
            const CartBar(),
          ],
        ),
      ),
    );
  }

  Widget _body(BuildContext context, List<Merchant> open, List<Merchant> closed) {
    if (_loading) {
      return const Padding(padding: EdgeInsets.all(K.s5), child: KSkeleton(height: 96));
    }
    if (_locationProblem != null) {
      return Padding(
        padding: const EdgeInsets.all(K.s5),
        child: KError(
          message: context.t(locationProblemKey(_locationProblem!)),
          retryLabel: context.t('common.retry'),
          onRetry: _load,
          icon: Icons.location_off_outlined,
        ),
      );
    }
    if (_error != null) {
      return Padding(
        padding: const EdgeInsets.all(K.s5),
        child: KError(
          message: context.tError(_error!.messageKey),
          retryLabel: context.t('common.retry'),
          onRetry: _load,
        ),
      );
    }
    if (_items.isEmpty) {
      return Padding(
        padding: const EdgeInsets.all(K.s5),
        child: KEmpty(
          title: context.t('food.empty'),
          message: context.t('food.empty.hint'),
          icon: Icons.storefront_outlined,
        ),
      );
    }
    return RefreshIndicator(
      onRefresh: _load,
      color: K.brand400,
      backgroundColor: K.surface2,
      child: ListView(
        padding: const EdgeInsets.fromLTRB(K.s5, K.s3, K.s5, K.s8),
        children: [
          const ActiveBanner(kind: ActiveKind.order),
          for (final m in open) _MerchantRow(merchant: m),
          if (closed.isNotEmpty) ...[
            const SizedBox(height: K.s4),
            KSectionHeader(context.t('food.closed')),
            const SizedBox(height: K.s3),
            for (final m in closed) _MerchantRow(merchant: m),
          ],
        ],
      ),
    );
  }
}

class _MerchantRow extends StatelessWidget {
  const _MerchantRow({required this.merchant});

  final Merchant merchant;

  @override
  Widget build(BuildContext context) {
    final locale = context.watch<AppState>().locale;
    final open = merchant.canOrder;
    return Padding(
      padding: const EdgeInsets.only(bottom: K.s2),
      child: KCard(
        onTap: () =>
            Navigator.of(context)
                .push(MaterialPageRoute<void>(builder: (_) => MenuScreen(merchant: merchant))),
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Opacity(
              opacity: open ? 1 : 0.55,
              child: KNetImage(
                url: merchant.logoUrl,
                width: 56,
                height: 56,
                fallbackIcon: Icons.storefront_outlined,
              ),
            ),
            const SizedBox(width: K.s3),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      Expanded(
                        child: Text(
                          merchant.name,
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: TextStyle(
                            fontSize: 16,
                            fontWeight: FontWeight.w700,
                            color: open ? K.text : K.muted,
                          ),
                        ),
                      ),
                      if (!open)
                        KBadge(context.t('food.closed'), tone: KTone.neutral)
                      else if (merchant.rating != null)
                        KBadge(merchant.rating!.toStringAsFixed(1), tone: KTone.ok),
                    ],
                  ),
                  const SizedBox(height: K.s1),
                  Text(
                    [
                      context.t(merchantTypeKey(merchant.type)),
                      if (merchant.cuisines.isNotEmpty) merchant.cuisines.first,
                      formatDistance(merchant.distanceM, locale: locale),
                    ].join(' · '),
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: const TextStyle(fontSize: 12, color: K.muted),
                  ),
                  const SizedBox(height: K.s2),
                  Text(
                    [
                      context.t('food.prep', {'min': '${merchant.prepTimeMin}'}),
                      context.t('food.delivery_fee', {
                        'amount': formatMinor(merchant.deliveryFeeMinor, locale: locale),
                      }),
                      context.t('food.min_order', {
                        'amount': formatMinor(merchant.minOrderMinor, locale: locale),
                      }),
                    ].join(' · '),
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: const TextStyle(fontSize: 12, color: K.textDim),
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}
