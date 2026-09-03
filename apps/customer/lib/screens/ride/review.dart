import 'package:flutter/material.dart';
import 'package:krejt_api/krejt_api.dart';
import 'package:krejt_design/krejt_design.dart';
import 'package:krejt_l10n/krejt_l10n.dart';
import 'package:provider/provider.dart';

import '../../state/app_state.dart';

/// Emrat janë ata të serverit (reviews.CustomerTags): një etiketë e panjohur refuzon gjithë
/// vlerësimin me 422, ndaj lista nuk shpiket këtu.
const reviewTags = ['clean_car', 'friendly', 'safe_driving', 'great_route'];

/// Vlerësimi pas udhëtimit. Kalimi lejohet: një vlerësim i detyruar është një vlerësim i rremë (§30).
class ReviewScreen extends StatefulWidget {
  const ReviewScreen({super.key, required this.ride});

  final Ride ride;

  @override
  State<ReviewScreen> createState() => _ReviewScreenState();
}

class _ReviewScreenState extends State<ReviewScreen> {
  final _comment = TextEditingController();
  final _tags = <String>{};

  int _rating = 5;
  bool _busy = false;
  String? _error;

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
    final thanks = context.t('ride.review.thanks');
    try {
      await context.read<AppState>().api.reviewRide(
        widget.ride.id,
        rating: _rating,
        tags: _tags.toList(),
        comment: _comment.text.trim(),
      );
      if (!mounted) return;
      messenger.showSnackBar(SnackBar(content: Text(thanks)));
      Navigator.of(context).pop();
    } on ApiError catch (e) {
      if (!mounted) return;
      setState(() => _error = context.tError(e.messageKey));
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final locale = context.watch<AppState>().locale;
    return Scaffold(
      backgroundColor: K.bg,
      appBar: AppBar(
        title: Text(context.t('ride.review.title')),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(context).pop(),
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
                  KRow(context.t('ride.dropoff'), widget.ride.dropoffAddress ?? '—'),
                  const KMoneyDivider(),
                  KMoneyRow(
                    context.t('ride.summary'),
                    widget.ride.priceMinor,
                    currency: widget.ride.currency,
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
                for (final tag in reviewTags)
                  FilterChip(
                    label: Text(context.t('ride.review.tag.$tag')),
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
            const SizedBox(height: K.s5),
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
