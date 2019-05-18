import os

from flask import Flask, g
from flask_restful import Resource, Api, reqparse
import markdown
import shelve

# create the app
app = Flask(__name__)

# create the API
api = Api(app)


def get_db():
    db = getattr(g, '_database', None)
    if db is None:
        db = g._database = shelve.open("data/probes.db")
    return db


@app.teardown_appcontext
def teardown_db(exception):
    db = getattr(g, '_database', None)
    if db is not None:
        db.close()


@app.route('/')
def index():
    """Present the API documentation"""

    # open the README file
    with open(os.path.dirname(app.root_path) + '/README.md', 'r') as md_file:
        # read the contents
        content = md_file.read()

        # convert to html
        return markdown.markdown(content)


class ProbeList(Resource):
    def get(self):
        shelf = get_db()
        keys = list(shelf.keys())

        probes = []

        for key in keys:
            probes.append(shelf[key])

        return {'message': 'Success', 'data': probes}, 200

    def post(self):
        parser = reqparse.RequestParser()

        parser.add_argument('endpoint', required=True)
        parser.add_argument('identifier', required=True)
        parser.add_argument('keys', required=True, action='append')
        parser.add_argument('versions', required=True, action='append')

        # Parse the arguments into an object
        args = parser.parse_args()

        # check if identifier exists
        id = args['identifier']

        shelf = get_db()
        if id in shelf:
            return {'message': 'Probe already exists', 'data': shelf[id]}, 200

        # create if it didn't
        shelf[args['identifier']] = args

        return {'message': 'Probe registered', 'data': args}, 201


class Probe(Resource):
    def get(self, identifier):
        shelf = get_db()

        # If the key does not exist in the data store, return a 404 error.
        if not (identifier in shelf):
            return {'message': 'Probe not found', 'data': {}}, 404

        return {'message': 'Probe found', 'data': shelf[identifier]}, 200

    def put(self, identifier):
        shelf = get_db()

        # If the key does not exist in the data store, return a 404 error.
        if not (identifier in shelf):
            return {'message': 'Probe not found', 'data': {}}, 404

        parser = reqparse.RequestParser()

        # probe will ignore the identifier to not create a mismatch between the identifier in the data and the URL
        parser.add_argument('endpoint')
        parser.add_argument('keys', action='append')
        parser.add_argument('versions', action='append')

        args = parser.parse_args()

        # don't return anything if no usable input data was provided
        vals = list(filter(None, args.values()))
        if len(vals) == 0:
            return '', 204

        # update it
        keys = list(args.keys())
        cfg = shelf[identifier]
        for key in keys:
            if args[key] is not None:
                cfg[key] = args[key]

        shelf[identifier] = cfg

        return {'message': 'Probe updated', 'data': cfg}, 200

    def delete(self, identifier):
        shelf = get_db()

        # If the key does not exist in the data store, return a 404 error.
        if not (identifier in shelf):
            return {'message': 'Probe not found', 'data': {}}, 404

        del shelf[identifier]
        return '', 204


# list all API routes
api.add_resource(ProbeList, '/probes')
api.add_resource(Probe, '/probes/<string:identifier>')
