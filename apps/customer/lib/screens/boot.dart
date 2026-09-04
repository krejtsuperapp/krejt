import 'package:flutter/material.dart';
import 'package:krejt_design/krejt_design.dart';
import 'package:krejt_l10n/krejt_l10n.dart';
import 'package:provider/provider.dart';

import '../state/app_state.dart';

/// Ekrani i nisjes. Mban logon e animuar derisa gjendja të vendoset, dhe shfaq rikthimin
/// kur nisja dështon për arsye që nuk janë sesioni (§55).
class BootScreen extends StatelessWidget {
  const BootScreen({super.key, this.showRetry = false});

  final bool showRetry;

  @override
  Widget build(BuildContext context) {
    final state = context.read<AppState>();
    return Scaffold(
      backgroundColor: K.bg,
      body: Stack(
        fit: StackFit.expand,
        children: [
          // Një dritë e lehtë neon në qendër, që sfondi i zi të mos duket i vdekur.
          const IgnorePointer(
            child: DecoratedBox(
              decoration: BoxDecoration(
                gradient: RadialGradient(
                  center: Alignment(0, -0.1),
                  radius: 0.75,
                  colors: [Color(0x2239FF14), Color(0x000D0D0D)],
                ),
              ),
            ),
          ),
          Center(
            child: Padding(
              padding: const EdgeInsets.all(K.s6),
              child: showRetry
                  ? KError(
                      title: context.t('state.error'),
                      message: context.tError(state.bootError?.messageKey ?? 'errors.internal'),
                      retryLabel: context.t('common.retry'),
                      onRetry: state.boot,
                    )
                  : Column(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        const KWordmark(size: 52),
                        const SizedBox(height: K.s3),
                        Text(
                          context.t('onboarding.subtitle'),
                          textAlign: TextAlign.center,
                          style: const TextStyle(fontSize: 14, color: K.muted),
                        ),
                        const SizedBox(height: K.s10),
                        const SizedBox(
                          width: 22,
                          height: 22,
                          child: CircularProgressIndicator(strokeWidth: 2.2, color: K.brand500),
                        ),
                      ],
                    ),
            ),
          ),
        ],
      ),
    );
  }
}
