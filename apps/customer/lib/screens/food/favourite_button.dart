import 'package:flutter/material.dart';
import 'package:krejt_api/krejt_api.dart';
import 'package:krejt_design/krejt_design.dart';
import 'package:krejt_l10n/krejt_l10n.dart';
import 'package:provider/provider.dart';

import '../../state/app_state.dart';

/// Zemra te një lokal.
///
/// Ndryshon menjëherë në ekran dhe e dërgon kërkesën pas: një prekje që pret rrjetin para se të
/// ndryshojë ngjyrën duket e prishur. Nëse serveri e refuzon, zemra kthehet aty ku ishte dhe
/// arsyeja tregohet — më mirë se një gënjeshtër e vogël që mbetet në ekran.
class FavouriteButton extends StatefulWidget {
  const FavouriteButton({super.key, required this.merchant, this.onChanged});

  final Merchant merchant;

  /// Njofton ekranin prind, që lista e të preferuarave të mos mbetet e vjetruar.
  final void Function(bool favourite)? onChanged;

  @override
  State<FavouriteButton> createState() => _FavouriteButtonState();
}

class _FavouriteButtonState extends State<FavouriteButton> {
  late bool _on = widget.merchant.favourite;
  bool _busy = false;

  @override
  void didUpdateWidget(FavouriteButton old) {
    super.didUpdateWidget(old);
    if (old.merchant.favourite != widget.merchant.favourite) _on = widget.merchant.favourite;
  }

  Future<void> _toggle() async {
    if (_busy) return;
    final want = !_on;
    setState(() {
      _on = want;
      _busy = true;
    });
    widget.onChanged?.call(want);

    final api = context.read<AppState>().api;
    final messenger = ScaffoldMessenger.of(context);
    final l10n = KL10n.of(context);
    try {
      if (want) {
        await api.addFavourite(widget.merchant.id);
      } else {
        await api.removeFavourite(widget.merchant.id);
      }
    } on ApiError catch (e) {
      if (!mounted) return;
      setState(() => _on = !want);
      widget.onChanged?.call(!want);
      messenger.showSnackBar(SnackBar(content: Text(l10n.error(e.messageKey))));
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) => Semantics(
    button: true,
    selected: _on,
    label: context.t(_on ? 'food.favourite.remove' : 'food.favourite.add'),
    child: IconButton(
      onPressed: _toggle,
      visualDensity: VisualDensity.compact,
      icon: Icon(
        _on ? Icons.favorite : Icons.favorite_border,
        size: 20,
        color: _on ? K.brand400 : K.line2,
      ),
    ),
  );
}
