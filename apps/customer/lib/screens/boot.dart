import 'package:flutter/material.dart';
import 'package:krejt_design/krejt_design.dart';
import 'package:krejt_l10n/krejt_l10n.dart';
import 'package:provider/provider.dart';

import '../state/app_state.dart';

/// Ekrani i nisjes. Mban logon derisa gjendja të vendoset, dhe shfaq rikthimin
/// kur nisja dështon për arsye që nuk janë sesioni (§55).
class BootScreen extends StatelessWidget {
  const BootScreen({super.key, this.showRetry = false});

  final bool showRetry;

  @override
  Widget build(BuildContext context) {
    final state = context.read<AppState>();
    return Scaffold(
      backgroundColor: K.bg,
      body: Center(
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
                    const _Wordmark(),
                    const SizedBox(height: K.s8),
                    KLoading(label: context.t('common.loading')),
                  ],
                ),
        ),
      ),
    );
  }
}

class _Wordmark extends StatelessWidget {
  const _Wordmark();

  @override
  Widget build(BuildContext context) => ShaderMask(
    shaderCallback: (r) => K.gradient.createShader(r),
    child: const Text(
      'KREJT',
      style: TextStyle(
        fontSize: 44,
        fontWeight: FontWeight.w800,
        letterSpacing: 6,
        color: Colors.white,
      ),
    ),
  );
}
