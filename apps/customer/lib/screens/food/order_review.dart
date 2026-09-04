import 'package:flutter/material.dart';
import 'package:krejt_api/krejt_api.dart';
import 'package:krejt_design/krejt_design.dart';
import 'package:krejt_l10n/krejt_l10n.dart';
import 'package:provider/provider.dart';

import '../../state/app_state.dart';

/// Etiketat e lokalit, në të njëjtën radhë si te serveri: fillimisht ato që lavdërojnë, pastaj
/// ato që ankohen. «Rrugë e mirë» nuk do të thotë gjë për një kuzhinë, ndaj lista është e vetja
/// dhe jo ajo e udhëtimit.
const merchantReviewTags = <String>[
  'tasty',
  'hot',
  'well_packed',
  'fast',
  'accurate',
  'cold',
  'late',
  'wrong_items',
  'small_portion',
];

/// Vlerësimi i një porosie.
///
/// Deri tani asnjë ekran nuk e shkruante `merchants.rating_sum`, ndaj çdo lokal mbetej përjetë pa
/// yll ndërsa lista e Ushqimit e kishte vendin gati për ta treguar. Një notë që askush nuk mund ta
/// japë nuk duhet të shfaqet fare — prandaj ky ekran erdhi para se ylli të fshihej.
class OrderReviewScreen extends StatefulWidget {
  const OrderReviewScreen({super.key, required this.order});

  final Order order;

  @override
  State<OrderReviewScreen> createState() => _OrderReviewScreenState();
}

class _OrderReviewScreenState extends State<OrderReviewScreen> {
  final _comment = TextEditingController();
  final _tags = <String>{};
  int _rating = 5;
  String? _error;
  bool _busy = false;

  @override
  void dispose() {
    _comment.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    setState(() {
      _busy = true;
      _error = null;
    });
    final messenger = ScaffoldMessenger.of(context);
    final thanks = context.t('order.review.thanks');
    try {
      await context.read<AppState>().api.reviewOrder(
        widget.order.id,
        rating: _rating,
        tags: _tags.toList(),
        comment: _comment.text.trim(),
      );
      if (!mounted) return;
      messenger.showSnackBar(SnackBar(content: Text(thanks)));
      Navigator.of(context).pop(true);
    } on ApiError catch (e) {
      if (!mounted) return;
      setState(() {
        _error = context.tError(e.messageKey);
        _busy = false;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final locale = context.watch<AppState>().locale;
    return Scaffold(
      backgroundColor: K.bg,
      appBar: AppBar(
        title: Text(context.t('order.review.title')),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(context).pop(false),
            child: Text(context.t('ride.review.skip')),
          ),
        ],
      ),
      body: SafeArea(
        child: ListView(
          padding: const EdgeInsets.all(K.s5),
          children: [
            KCard(
              child: Column(
                children: [
                  KRow(context.t('order.merchant'), widget.order.merchantName),
                  const KMoneyDivider(),
                  KMoneyRow(
                    context.t('order.total'),
                    widget.order.totalMinor,
                    currency: widget.order.currency,
                    locale: locale,
                    total: true,
                  ),
                ],
              ),
            ),
            const SizedBox(height: K.s6),
            Row(
              mainAxisAlignment: MainAxisAlignment.center,
              children: [
                for (var i = 1; i <= 5; i++)
                  IconButton(
                    iconSize: 36,
                    onPressed: () => setState(() => _rating = i),
                    icon: Icon(
                      i <= _rating ? Icons.star_rounded : Icons.star_outline_rounded,
                      color: i <= _rating ? K.warn : K.line2,
                    ),
                  ),
              ],
            ),
            const SizedBox(height: K.s4),
            Wrap(
              spacing: K.s2,
              runSpacing: K.s2,
              alignment: WrapAlignment.center,
              children: [
                for (final tag in merchantReviewTags)
                  FilterChip(
                    label: Text(context.t('order.review.tag.$tag')),
                    selected: _tags.contains(tag),
                    onSelected: (on) => setState(() {
                      if (on) {
                        _tags.add(tag);
                      } else {
                        _tags.remove(tag);
                      }
                    }),
                  ),
              ],
            ),
            const SizedBox(height: K.s5),
            KField(
              label: context.t('ride.review.comment'),
              controller: _comment,
              maxLength: 500,
              maxLines: 4,
            ),
            if (_error != null) ...[
              const SizedBox(height: K.s3),
              Text(_error!, style: const TextStyle(fontSize: 13, color: K.danger)),
            ],
            const SizedBox(height: K.s6),
            KButton(
              label: context.t('ride.review.submit'),
              busy: _busy,
              onPressed: _busy ? null : _submit,
            ),
          ],
        ),
      ),
    );
  }
}
