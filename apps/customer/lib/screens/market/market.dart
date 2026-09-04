import 'dart:async';

import 'package:flutter/material.dart';
import 'package:krejt_api/krejt_api.dart';
import 'package:krejt_design/krejt_design.dart';
import 'package:krejt_l10n/krejt_l10n.dart';
import 'package:provider/provider.dart';

import '../../services/location.dart';
import '../../state/app_state.dart';
import '../active_banner.dart';
import '../food/cart_bar.dart';
import '../food/favourite_button.dart';
import '../food/menu.dart';

/// Marketi si shërbim më vete, jo si Ushqimi me një flamur.
///
/// Deri tani të dyja hapnin të njëjtin ekran me `mode: market`, dhe kjo dukej e lirë sepse te
/// serveri janë i njëjti tabelë tregtarësh me një `type` tjetër. Por njeriu që kërkon darkë dhe
/// njeriu që kërkon qumësht nuk mendojnë njësoj: i pari zgjedh një lokal e pastaj një pjatë; i
/// dyti niset nga kategoria — ushqimore, farmaci, dyqan — dhe pastaj zgjedh se ku ta blejë.
///
/// Prandaj Marketi niset nga kategoritë dhe jo nga lista e tregtarëve, ka fjalët e veta dhe
/// gjendjet e veta bosh, edhe pse poshtë përdor të njëjtin model të dhënash dhe të njëjtën shportë.
class MarketScreen extends StatefulWidget {
  const MarketScreen({super.key});

  @override
  State<MarketScreen> createState() => _MarketScreenState();
}

/// Kategoritë e Marketit, në radhën si i kërkon njeriu më shpesh.
const _categories = <({String type, String labelKey, IconData icon})>[
  (type: 'grocery', labelKey: 'market.grocery', icon: Icons.local_grocery_store_outlined),
  (type: 'pharmacy', labelKey: 'market.pharmacy', icon: Icons.medical_services_outlined),
  (type: 'store', labelKey: 'market.store', icon: Icons.storefront_outlined),
];

class _MarketScreenState extends State<MarketScreen> {
  static const _location = LocationService();

  final _search = TextEditingController();
  Timer? _debounce;

  List<Merchant> _items = const [];
  String _type = 'grocery';
  LatLng? _at;
  LocationProblem? _problem;
  ApiError? _error;
  bool _loading = true;

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

  /// Kërkimi pret gjysmë sekonde pas shkronjës së fundit, që të mos dërgojë një kërkesë për çdo
  /// prekje — njësoj si te Ushqimi.
  void _onQueryChanged(String _) {
    _debounce?.cancel();
    _debounce = Timer(const Duration(milliseconds: 500), _load);
  }

  Future<void> _load() async {
    if (mounted) setState(() => _loading = true);
    // Vendndodhja kërkohet këtu e jo te ballina: leja duhet kërkuar kur përdoruesi vërtet po
    // kërkon një dyqan, jo sa herë hap aplikacionin.
    var at = _at;
    if (at == null) {
      final position = await _location.current();
      if (!mounted) return;
      if (!position.isOk) {
        setState(() {
          _problem = position.problem;
          _loading = false;
        });
        return;
      }
      at = position.point;
    }
    try {
      final q = _search.text.trim();
      final items = await context.read<AppState>().api.merchants(
        lat: at!.lat,
        lng: at.lng,
        type: _type,
        query: q.isEmpty ? null : q,
        limit: 30,
      );
      if (!mounted) return;
      setState(() {
        _at = at;
        _items = items;
        _problem = null;
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

  void _pick(String type) {
    if (_type == type) return;
    setState(() {
      _type = type;
      _items = const [];
    });
    _load();
  }

  @override
  Widget build(BuildContext context) {
    final open = _items.where((m) => m.canOrder).toList();
    final closed = _items.where((m) => !m.canOrder).toList();

    return Scaffold(
      backgroundColor: K.bg,
      appBar: AppBar(title: Text(context.t('home.services.market'))),
      body: SafeArea(
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Padding(
              padding: const EdgeInsets.fromLTRB(K.s5, K.s2, K.s5, K.s3),
              child: KField(
                label: context.t('common.search'),
                controller: _search,
                hint: context.t('market.search'),
                onChanged: _onQueryChanged,
                onSubmitted: (_) => _load(),
              ),
            ),
            Padding(
              padding: const EdgeInsets.fromLTRB(K.s5, 0, K.s5, K.s3),
              child: Row(
                children: [
                  for (final c in _categories) ...[
                    Expanded(
                      child: _CategoryTile(
                        icon: c.icon,
                        label: context.t(c.labelKey),
                        selected: _type == c.type,
                        onTap: () => _pick(c.type),
                      ),
                    ),
                    if (c != _categories.last) const SizedBox(width: K.s3),
                  ],
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
    if (_problem != null) {
      return Padding(
        padding: const EdgeInsets.all(K.s5),
        child: KError(
          message: context.t(locationProblemKey(_problem!)),
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
        // Gjendja bosh e emërton kategorinë: 'asnjë dyqan' kur je te Farmacia e lë përdoruesin
        // të pyesë nëse kërkoi gabim, ndërsa problemi është vetëm te ajo kategori.
        child: KEmpty(
          title: context.t('market.empty.in', {
            'category': context.t(_categories.firstWhere((c) => c.type == _type).labelKey),
          }),
          message: context.t('market.empty.hint'),
          icon: Icons.storefront_outlined,
        ),
      );
    }
    return RefreshIndicator(
      onRefresh: _load,
      color: K.brand400,
      backgroundColor: K.surface2,
      child: ListView(
        padding: const EdgeInsets.fromLTRB(K.s5, 0, K.s5, K.s8),
        children: [
          const ActiveBanner(kind: ActiveKind.order),
          for (final m in open) _StoreRow(merchant: m),
          if (closed.isNotEmpty) ...[
            const SizedBox(height: K.s4),
            KSectionHeader(context.t('food.closed')),
            const SizedBox(height: K.s3),
            for (final m in closed) _StoreRow(merchant: m),
          ],
        ],
      ),
    );
  }
}

class _CategoryTile extends StatelessWidget {
  const _CategoryTile({
    required this.icon,
    required this.label,
    required this.selected,
    required this.onTap,
  });

  final IconData icon;
  final String label;
  final bool selected;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) => Semantics(
    button: true,
    selected: selected,
    child: Material(
      color: selected ? K.brand500.withValues(alpha: 0.14) : K.surface,
      borderRadius: BorderRadius.circular(K.rMd),
      child: InkWell(
        borderRadius: BorderRadius.circular(K.rMd),
        onTap: onTap,
        child: Container(
          padding: const EdgeInsets.symmetric(vertical: K.s4, horizontal: K.s2),
          decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(K.rMd),
            border: Border.all(color: selected ? K.brand500 : K.line2),
          ),
          child: Column(
            children: [
              Icon(icon, size: 22, color: selected ? K.brand400 : K.textDim),
              const SizedBox(height: K.s2),
              Text(
                label,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: TextStyle(
                  fontSize: 12,
                  fontWeight: FontWeight.w600,
                  color: selected ? K.brand400 : K.textDim,
                ),
              ),
            ],
          ),
        ),
      ),
    ),
  );
}

/// Rreshti i dyqanit: kartë e plotë, jo kartë e ngushtë. KMerchantCard ka gjerësi fikse dhe bën
/// punë te karuselet horizontale; në një listë vertikale do të linte gjysmën e ekranit bosh.
class _StoreRow extends StatelessWidget {
  const _StoreRow({required this.merchant});

  final Merchant merchant;

  @override
  Widget build(BuildContext context) {
    final locale = context.watch<AppState>().locale;
    final open = merchant.canOrder;
    return Padding(
      padding: const EdgeInsets.only(bottom: K.s2),
      child: KCard(
        onTap: open
            ? () =>
                  Navigator.of(context)
                      .push(MaterialPageRoute<void>(builder: (_) => MenuScreen(merchant: merchant)))
            : null,
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Opacity(
              opacity: open ? 1 : 0.55,
              child: KNetImage(
                url: merchant.coverUrl ?? merchant.logoUrl,
                height: 64,
                width: 64,
                radius: K.rSm,
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
                      FavouriteButton(merchant: merchant),
                    ],
                  ),
                  const SizedBox(height: K.s1),
                  Text(
                    [
                      if (merchant.addressLine1.isNotEmpty) merchant.addressLine1,
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
