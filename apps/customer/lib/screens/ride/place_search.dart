import 'dart:async';

import 'package:flutter/material.dart';
import 'package:krejt_api/krejt_api.dart';
import 'package:krejt_design/krejt_design.dart';
import 'package:krejt_l10n/krejt_l10n.dart';
import 'package:provider/provider.dart';

import '../../services/location.dart';
import '../../state/app_state.dart';
import '../account/addresses.dart';

/// Një vend i zgjedhur: pika me koordinata të vërteta dhe emri që i shfaqet përdoruesit.
class PickedPlace {
  const PickedPlace({required this.point, required this.label});

  final LatLng point;
  final String label;
}

/// Fleta e kërkimit të një vendi: kërkim me shkrim (serveri), vendndodhja ime, adresat e ruajtura.
/// Përdoret nga pakot (marrja/dorëzimi) dhe kudo ku duhet një pikë me koordinata.
Future<PickedPlace?> showPlaceSearch(BuildContext context, {required String title}) =>
    showModalBottomSheet<PickedPlace>(
      context: context,
      isScrollControlled: true,
      backgroundColor: Colors.transparent,
      builder: (_) => _PlaceSearchSheet(title: title),
    );

class _PlaceSearchSheet extends StatefulWidget {
  const _PlaceSearchSheet({required this.title});

  final String title;

  @override
  State<_PlaceSearchSheet> createState() => _PlaceSearchSheetState();
}

class _PlaceSearchSheetState extends State<_PlaceSearchSheet> {
  static const _debounce = Duration(milliseconds: 350);

  final _query = TextEditingController();
  List<Place> _results = const [];
  List<Address> _addresses = const [];
  bool _searching = false;
  bool _locating = false;
  Timer? _timer;
  int _gen = 0;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _loadAddresses());
  }

  @override
  void dispose() {
    _timer?.cancel();
    _query.dispose();
    super.dispose();
  }

  Future<void> _loadAddresses() async {
    try {
      final items = await context.read<AppState>().api.addresses();
      if (mounted) setState(() => _addresses = items);
    } on ApiError {
      // Kërkimi mbetet rruga kryesore.
    }
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
    try {
      final items = await context.read<AppState>().api.searchPlaces(q);
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

  Future<void> _useCurrent() async {
    setState(() => _locating = true);
    final result = await const LocationService().current();
    if (!mounted) return;
    setState(() => _locating = false);
    if (!result.isOk) {
      ScaffoldMessenger.of(context)
          .showSnackBar(SnackBar(content: Text(context.t(locationProblemKey(result.problem!)))));
      return;
    }
    var label = context.t('location.use_current');
    try {
      final place = await context.read<AppState>().api.reversePlace(result.point!);
      if (place != null) label = place.name;
    } on ApiError {
      // Emri është rehati, jo kusht.
    }
    if (!mounted) return;
    Navigator.of(context).pop(PickedPlace(point: result.point!, label: label));
  }

  @override
  Widget build(BuildContext context) {
    final query = _query.text.trim();
    // Hapësira që mbetet vërtet, jo ekrani i plotë: fusha ka autofocus, ndaj tastiera është aty
    // që nga çelja dhe merr rreth dy të pestat. Matja e ekranit të plotë e bënte listën të
    // kërkonte më shumë se sa ekziston.
    final media = MediaQuery.of(context);
    final free = media.size.height - media.viewInsets.bottom;
    return KSheet(
      title: widget.title,
      padding: const EdgeInsets.fromLTRB(K.s5, K.s2, K.s5, K.s4),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          KField(
            label: context.t('ride.search.hint'),
            controller: _query,
            prefix: const Icon(Icons.search, size: 20, color: K.muted),
            autofocus: true,
            textInputAction: TextInputAction.search,
            onChanged: _onQuery,
          ),
          const SizedBox(height: K.s3),
          ConstrainedBox(
            constraints: BoxConstraints(maxHeight: free * 0.55),
            child: ListView(
              shrinkWrap: true,
              children: [
                if (query.length < 2) ...[
                  _Row(
                    icon: Icons.my_location,
                    title: context.t('location.use_current'),
                    busy: _locating,
                    onTap: _locating ? null : _useCurrent,
                  ),
                  for (final a in _addresses)
                    _Row(
                      icon: addressIcon(a.label),
                      title: a.name ?? context.t(addressLabelKey(a.label)),
                      subtitle: '${a.line1}, ${a.city}',
                      onTap: () =>
                          Navigator.of(context)
                              .pop(PickedPlace(point: LatLng(a.lat, a.lng), label: a.line1)),
                    ),
                ] else if (_searching)
                  const KSkeleton(height: 52, count: 3)
                else if (_results.isEmpty)
                  Padding(
                    padding: const EdgeInsets.symmetric(vertical: K.s4),
                    child: Text(
                      context.t('ride.search.empty'),
                      textAlign: TextAlign.center,
                      style: const TextStyle(fontSize: 13, color: K.muted),
                    ),
                  )
                else
                  for (final p in _results)
                    _Row(
                      icon: Icons.place_outlined,
                      title: p.name,
                      subtitle: p.subtitle.isEmpty ? null : p.subtitle,
                      onTap: () =>
                          Navigator.of(context).pop(PickedPlace(point: p.point, label: p.name)),
                    ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class _Row extends StatelessWidget {
  const _Row({
    required this.icon,
    required this.title,
    required this.onTap,
    this.subtitle,
    this.busy = false,
  });

  final IconData icon;
  final String title;
  final String? subtitle;
  final VoidCallback? onTap;
  final bool busy;

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
            child: busy
                ? const SizedBox(
                    width: 16,
                    height: 16,
                    child: CircularProgressIndicator(strokeWidth: 2, color: K.brand500),
                  )
                : Icon(icon, size: 18, color: K.textDim),
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
        ],
      ),
    ),
  );
}
