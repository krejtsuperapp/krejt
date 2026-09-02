import { ApiError } from './api';

/// Tekstet e gabimeve sipas `message_key` që dërgon serveri. Kuzhina nuk sheh kurrë
/// tekst teknik; një çelës i panjohur bie te mesazhi i përgjithshëm (§55).
const messages: Record<string, string> = {
  'errors.internal': 'Diçka shkoi keq. Provo përsëri.',
  'errors.offline': 'Nuk ka lidhje me serverin.',
  'errors.unauthorized': 'Sesioni skadoi. Kyçu përsëri.',
  'errors.forbidden': 'Nuk ke të drejta për këtë veprim.',
  'errors.not_found': 'Nuk u gjet.',
  'errors.validation': 'Kontrollo të dhënat e shkruara.',
  'errors.rate_limited': 'Shumë përpjekje. Prit pak.',
  'errors.otp_invalid': 'Kodi nuk përputhet ose ka skaduar.',
  'errors.order_conflict': 'Porosia ndryshoi ndërkohë. Rifresko listën.',
};

export function errorText(e: unknown): string {
  if (e instanceof ApiError) return messages[e.messageKey] ?? messages['errors.internal'];
  return messages['errors.offline'];
}
