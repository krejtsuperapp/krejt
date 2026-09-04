import 'dart:async';

import 'package:flutter/material.dart';
import 'package:krejt_api/krejt_api.dart' hide Place;
import 'package:krejt_api/krejt_api.dart' as api show Place;
import 'package:krejt_design/krejt_design.dart';
import 'package:krejt_l10n/krejt_l10n.dart';
import 'package:krejt_map/krejt_map.dart';
import 'package:provider/provider.dart';

import '../../services/location.dart';
import '../../state/app_state.dart';
import '../account/addresses.dart';
import '../active_banner.dart';
import 'map_scaffold.dart';
import 'quote.dart';

/// Një pikë e zgjedhur nga përdoruesi, me emrin që i shfaqet.
class Place {
  const Place({required this.point, required this.label});

  final LatLng point;
  final String label;
}

enum _Editing { pickup, dropoff }

/// Zgjedhja e nisjes dhe e destinacionit mbi hartë. Nisja merret nga pajisja (dhe i vihet
/// adresa nga serveri); destinacioni kërkohet me shkrim — kërkimi kalon nga serveri, që
/// çdo rezultat të vijë me koordinata të vërteta (§17). Adresat e ruajtura dhe të fundit
/// mbeten një prekje larg.
class DestinationScreen extends StatefulWidget {
  const DestinationScreen({super.key});

  @override
  State<DestinationScreen> createState() => _DestinationScreenState();
}

class _DestinationScreenState extends State<DestinationScreen> {
  static const _location = LocationService();
  static const _debounce = Duration(milliseconds: 350);

  final _query = TextEditingController();
  final _focus = FocusNode();

  Place? _pickup;
  Place? _dropoff;
  List<Address> _addresses = const [];
  List<api.Place> _results = const [];
  _Editing _editing = _Editing.dropoff;
  bool _locating = true;
  bool _searching = false;
  String? _locationError;
  Timer? _timer;
  int _gen = 0;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      _locate();
      _loadAddresses();
    });
  }

  @override
  void dispose() {
    _timer?.cancel();
    _query.dispose();
    _focus.dispose();
    super.dispose();
  }

  Future<void> _locate() async {
    if (mounted) setState(() => _locating = true);
    final result = await _location.current();
    if (!mounted) return;
    setState(() {
      _locating = false;
      if (result.isOk) {
        _pickup = Place(point: result.point!, label: context.t('location.use_current'));
        _locationError = null;
      } else {
        _locationError = context.t(locationProblemKey(result.problem!));
      }
    });
    if (result.isOk) unawaited(_labelPickup(result.point!));
  }

  /// Adresa e vërtetë e pikës së GPS-it; pa të, mbetet "vendndodhja ime".
  Future<void> _labelPickup(LatLng point) async {
    try {
      final place = await context.read<AppState>().api.reversePlace(point);
      if (!mounted || place == null || _pickup?.point != point) return;
      setState(() => _pickup = Place(point: point, label: place.name));
    } on ApiError {
      // Emri është rehati, jo kusht.
    }
  }

  Future<void> _loadAddresses() async {
    try {
      final items = await context.read<AppState>().api.addresses();
      if (mounted) setState(() => _addresses = items);
    } on ApiError {
      // Adresat mungojnë; kërkimi dhe destinacionet e fundit mbeten.
    }
  }

  /// Destinacionet e fundit dalin nga historiku: adresa e zbritjes e udhëtimeve të mbyllura.
  List<Place> get _recent {
    final seen = <String>{};
    final out = <Place>[];
    for (final r in context.read<AppState>().recentRides) {
      final label = r.dropoffAddress;
      if (label == null || label.isEmpty || !seen.add(label)) continue;
      out.add(Place(point: r.dropoff, label: label));
      if (out.length == 4) break;
    }
    return out;
  }

  void _onQuery(String text) {
    _timer?.cancel();
    final q = text.trim();
    if (q.length < 2) {
      setState(() {
        _results = const [];
        _searching = false;
      });
      return;
    }
    setState(() => _searching = true);
    _timer = Timer(_debounce, () => _search(q));
  }

  Future<void> _search(String q) async {
    final gen = ++_gen;
    final near = _pickup?.point ?? _dropoff?.point;
    try {
      final items = await context.read<AppState>().api.searchPlaces(q, near: near);
      if (!mounted || gen != _gen) return;
      setState(() {
        _results = items;
        _searching = false;
      });
    } on ApiError {
      if (!mounted || gen != _gen) return;
      setState(() {
        _results = const [];
        _searching = false;
      });
    }
  }

  void _choose(Place place) {
    setState(() {
      if (_editing == _Editing.pickup) {
        _pickup = place;
        _locationError = null;
        _editing = _Editing.dropoff;
      } else {
        _dropoff = place;
      }
      _query.clear();
      _results = const [];
      _searching = false;
    });
    if (_dropoff == null) {
      _focus.requestFocus();
    } else {
      _focus.unfocus();
    }
  }

  void _edit(_Editing field) {
    setState(() {
      _editing = field;
      _query.clear();
      _results = const [];
    });
    _focus.requestFocus();
  }

  Future<void> _continue() async {
    final pickup = _pickup;
    final dropoff = _dropoff;
    if (pickup == null || dropoff == null) return;
    await Navigator.of(context).push(
      MaterialPageRoute<void>(
        builder: (_) => QuoteScreen(pickup: pickup, dropoff: dropoff),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final pickup = _pickup;
    final dropoff = _dropoff;
    final query = _query.text.trim();
    return MapScaffold.panel(
      title: context.t('home.services.ride'),
      markers: [
        if (pickup != null) markerOf(pickup.point.lat, pickup.point.lng, MapMarkerKind.pickup),
        if (dropoff != null) markerOf(dropoff.point.lat, dropoff.point.lng, MapMarkerKind.dropoff),
      ],
      panel: Padding(
        padding: EdgeInsets.only(bottom: MediaQuery.viewInsetsOf(context).bottom),
        child: SafeArea(
          top: false,
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              const Center(child: KSheetHandle()),
              Padding(
                padding: const EdgeInsets.fromLTRB(K.s5, K.s2, K.s5, 0),
                child: _Fields(
                  pickup: pickup,
                  dropoff: dropoff,
                  locating: _locating,
                  editing: _editing,
                  controller: _query,
                  focus: _focus,
                  onQuery: _onQuery,
                  onEdit: _edit,
                ),
              ),
              if (_locationError != null && pickup == null)
                Padding(
                  padding: const EdgeInsets.fromLTRB(K.s5, K.s3, K.s5, 0),
                  child: _LocationProblem(message: _locationError!, onRetry: _locate),
                ),
              ConstrainedBox(
                constraints: BoxConstraints(maxHeight: MediaQuery.sizeOf(context).height * 0.36),
                child: ListView(
                  shrinkWrap: true,
                  padding: const EdgeInsets.fromLTRB(K.s5, K.s3, K.s5, K.s2),
                  children: query.length >= 2 ? _searchRows(context) : _suggestionRows(context),
                ),
              ),
              Padding(
                padding: const EdgeInsets.fromLTRB(K.s5, K.s2, K.s5, K.s4),
                child: KButton(
                  label: context.t('common.continue'),
                  icon: Icons.arrow_forward,
                  onPressed: (pickup != null && dropoff != null) ? _continue : null,
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }

  List<Widget> _searchRows(BuildContext context) {
    if (_searching) return const [KSkeleton(height: 52, count: 3)];
    if (_results.isEmpty) {
      return [
        Padding(
          padding: const EdgeInsets.symmetric(vertical: K.s4),
          child: Text(
            context.t('ride.search.empty'),
            textAlign: TextAlign.center,
            style: const TextStyle(fontSize: 13, color: K.muted),
          ),
        ),
      ];
    }
    return [
      for (final p in _results)
        _PlaceRow(
          icon: _iconFor(p.kind),
          title: p.name,
          subtitle: p.subtitle.isEmpty ? null : p.subtitle,
          onTap: () => _choose(Place(point: p.point, label: p.name)),
        ),
    ];
  }

  List<Widget> _suggestionRows(BuildContext context) {
    final recent = _recent;
    final rows = <Widget>[];
    rows.add(const ActiveBanner(kind: ActiveKind.ride));
    if (_addresses.isNotEmpty) {
      rows.add(KSectionHeader(context.t('ride.search.saved')));
      rows.add(const SizedBox(height: K.s2));
      for (final a in _addresses) {
        rows.add(
          _PlaceRow(
            icon: addressIcon(a.label),
            title: a.name ?? context.t(addressLabelKey(a.label)),
            subtitle: '${a.line1}, ${a.city}',
            onTap: () => _choose(Place(point: LatLng(a.lat, a.lng), label: a.line1)),
          ),
        );
      }
    }
    if (recent.isNotEmpty) {
      if (rows.isNotEmpty) rows.add(const SizedBox(height: K.s3));
      rows.add(KSectionHeader(context.t('ride.recent')));
      rows.add(const SizedBox(height: K.s2));
      for (final p in recent) {
        rows.add(_PlaceRow(icon: Icons.history, title: p.label, onTap: () => _choose(p)));
      }
    }
    if (rows.isEmpty) {
      rows.add(
        Padding(
          padding: const EdgeInsets.symmetric(vertical: K.s3),
          child: Row(
            children: [
              const Icon(Icons.search, size: 18, color: K.muted),
              const SizedBox(width: K.s2),
              Expanded(
                child: Text(
                  context.t('account.address.empty.hint'),
                  style: const TextStyle(fontSize: 13, color: K.muted),
                ),
              ),
              KTextLink(
                label: context.t('account.address.add'),
                onPressed: () async {
                  await Navigator.of(context)
                      .push<bool>(MaterialPageRoute(builder: (_) => const AddAddressScreen()));
                  await _loadAddresses();
                },
              ),
            ],
          ),
        ),
      );
    }
    return rows;
  }

  static IconData _iconFor(String kind) {
    switch (kind) {
      case 'poi':
        return Icons.place_outlined;
      case 'address':
      case 'street':
        return Icons.signpost_outlined;
      case 'place':
      case 'locality':
      case 'neighborhood':
        return Icons.location_city_outlined;
    }
    return Icons.location_on_outlined;
  }
}

/// Dy rreshtat e rrugës: cili është në përpunim mban fushën e shkrimit, tjetri tregon zgjedhjen.
class _Fields extends StatelessWidget {
  const _Fields({
    required this.pickup,
    required this.dropoff,
    required this.locating,
    required this.editing,
    required this.controller,
    required this.focus,
    required this.onQuery,
    required this.onEdit,
  });

  final Place? pickup;
  final Place? dropoff;
  final bool locating;
  final _Editing editing;
  final TextEditingController controller;
  final FocusNode focus;
  final ValueChanged<String> onQuery;
  final ValueChanged<_Editing> onEdit;

  @override
  Widget build(BuildContext context) {
    return Container(
      decoration: BoxDecoration(
        color: K.surface2,
        borderRadius: BorderRadius.circular(K.rLg),
        border: Border.all(color: K.line),
      ),
      padding: const EdgeInsets.symmetric(horizontal: K.s4, vertical: K.s2),
      child: Column(
        children: [
          _FieldRow(
            color: K.brand500,
            square: false,
            active: editing == _Editing.pickup,
            hint: context.t('ride.search.pickup_hint'),
            value: locating ? context.t('ride.pickup.locating') : pickup?.label,
            controller: controller,
            focus: focus,
            onQuery: onQuery,
            onTap: () => onEdit(_Editing.pickup),
            trailing: pickup != null && editing != _Editing.pickup
                ? const Icon(Icons.edit_outlined, size: 18, color: K.muted)
                : null,
          ),
          const Divider(height: 1, color: K.line),
          _FieldRow(
            color: K.info,
            square: true,
            active: editing == _Editing.dropoff,
            hint: context.t('ride.search.hint'),
            value: dropoff?.label,
            controller: controller,
            focus: focus,
            onQuery: onQuery,
            onTap: () => onEdit(_Editing.dropoff),
            trailing: dropoff != null && editing != _Editing.dropoff
                ? const Icon(Icons.check_circle, size: 18, color: K.brand400)
                : null,
          ),
        ],
      ),
    );
  }
}

class _FieldRow extends StatelessWidget {
  const _FieldRow({
    required this.color,
    required this.square,
    required this.active,
    required this.hint,
    required this.value,
    required this.controller,
    required this.focus,
    required this.onQuery,
    required this.onTap,
    this.trailing,
  });

  final Color color;
  final bool square;
  final bool active;
  final String hint;
  final String? value;
  final TextEditingController controller;
  final FocusNode focus;
  final ValueChanged<String> onQuery;
  final VoidCallback onTap;
  final Widget? trailing;

  @override
  Widget build(BuildContext context) {
    final dot = Container(
      width: 10,
      height: 10,
      decoration: BoxDecoration(
        color: color,
        borderRadius: BorderRadius.circular(square ? 2 : K.rFull),
        boxShadow: [BoxShadow(color: color.withValues(alpha: 0.55), blurRadius: 8)],
      ),
    );
    const style = TextStyle(fontSize: 15, fontWeight: FontWeight.w600, color: K.text);
    return SizedBox(
      height: 46,
      child: Row(
        children: [
          dot,
          const SizedBox(width: K.s3),
          Expanded(
            child: active
                ? TextField(
                    controller: controller,
                    focusNode: focus,
                    onChanged: onQuery,
                    textInputAction: TextInputAction.search,
                    style: style,
                    decoration: InputDecoration(
                      isDense: true,
                      border: InputBorder.none,
                      hintText: value ?? hint,
                      hintStyle: TextStyle(
                        fontSize: 15,
                        fontWeight: FontWeight.w500,
                        color: value == null ? K.muted : K.textDim,
                      ),
                    ),
                  )
                : InkWell(
                    onTap: onTap,
                    child: Align(
                      alignment: Alignment.centerLeft,
                      child: Text(
                        value ?? hint,
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: value == null ? style.copyWith(color: K.muted) : style,
                      ),
                    ),
                  ),
          ),
          if (trailing != null) ...[const SizedBox(width: K.s2), trailing!],
        ],
      ),
    );
  }
}

class _LocationProblem extends StatelessWidget {
  const _LocationProblem({required this.message, required this.onRetry});

  final String message;
  final VoidCallback onRetry;

  @override
  Widget build(BuildContext context) => Container(
    padding: const EdgeInsets.symmetric(horizontal: K.s3, vertical: K.s2),
    decoration: BoxDecoration(color: K.warnBg, borderRadius: BorderRadius.circular(K.rSm)),
    child: Row(
      children: [
        const Icon(Icons.location_off_outlined, size: 18, color: K.warn),
        const SizedBox(width: K.s2),
        Expanded(
          child: Text(message, style: const TextStyle(fontSize: 12, color: K.textDim)),
        ),
        KTextLink(label: context.t('common.retry'), onPressed: onRetry),
      ],
    ),
  );
}

class _PlaceRow extends StatelessWidget {
  const _PlaceRow({required this.icon, required this.title, required this.onTap, this.subtitle});

  final IconData icon;
  final String title;
  final String? subtitle;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) => InkWell(
    onTap: onTap,
    borderRadius: BorderRadius.circular(K.rSm),
    child: Padding(
      padding: const EdgeInsets.symmetric(vertical: K.s2),
      child: Row(
        children: [
          Container(
            width: 36,
            height: 36,
            alignment: Alignment.center,
            decoration: BoxDecoration(
              color: K.surface2,
              borderRadius: BorderRadius.circular(K.rSm),
            ),
            child: Icon(icon, size: 18, color: K.textDim),
          ),
          const SizedBox(width: K.s3),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  title,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: const TextStyle(fontSize: 14, fontWeight: FontWeight.w600, color: K.text),
                ),
                if (subtitle != null)
                  Text(
                    subtitle!,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: const TextStyle(fontSize: 12, color: K.muted),
                  ),
              ],
            ),
          ),
          const Icon(Icons.north_west, size: 16, color: K.line2),
        ],
      ),
    ),
  );
}
