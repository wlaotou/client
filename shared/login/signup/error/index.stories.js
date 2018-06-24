// @flow
import * as React from 'react'
import Error from '.'
import {action, storiesOf} from '../../../stories/storybook'

const load = () => {
  storiesOf('Signup', module).add('Error', () => (
    <Error error="This is an error" onBack={action('onBack')} />
  ))
}

export default load
