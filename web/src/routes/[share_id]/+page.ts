import { error } from '@sveltejs/kit';
import type { PageLoad } from './$types';

export const load: PageLoad = ({ params }) => {
  const { share_id } = params;

  // Validate share_id format
  // Share IDs are exactly 12 alphanumeric characters (a-z, A-Z, 0-9)
  const validShareIdPattern = /^[a-zA-Z0-9]{12}$/;

  if (!validShareIdPattern.test(share_id)) {
    throw error(404, {
      message: 'Invalid share link format'
    });
  }

  return {
    share_id
  };
};
