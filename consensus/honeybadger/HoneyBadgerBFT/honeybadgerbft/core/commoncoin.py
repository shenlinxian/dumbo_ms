import logging

#from honeybadgerbft.crypto.threshsig.boldyreva import serialize
from collections import defaultdict
from gevent import Greenlet
from gevent.queue import Queue
import hashlib

logger = logging.getLogger(__name__)


class CommonCoinFailureException(Exception):
    """Raised for common coin failures."""
    pass


def hash(x):
    return hashlib.sha256(x).digest()


def fake_coin(sid):
    """my function: return a fake coin
    :param sid: a unique instance id
    """

    def getCoin(round):
        """Gets a coin.

        :param round: the epoch/round.
        :returns: a coin.

        """
        seed="%s%d" % (sid,round)
        b = seed.encode('utf-8')
        bit = hash(b)[0] % 2
        return bit

    return getCoin
    

